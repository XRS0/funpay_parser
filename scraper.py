"""Funpay listing scraper focused on the ChatGPT category."""
import re
import time
from urllib.parse import urljoin, urlencode

import requests
from bs4 import BeautifulSoup

from config import FUNPAY_BASE_URL, FUNPAY_LANG, MAX_PAGES, PROXY_PROXIES
from models import Listing


HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/125.0.0.0 Safari/537.36"
    ),
    "Accept-Language": "en-US,en;q=0.9,ru;q=0.8",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
}


_SESSION = requests.Session()
_SESSION.headers.update(HEADERS)
if PROXY_PROXIES:
    _SESSION.proxies.update(PROXY_PROXIES)
    print(f"[scraper] Using proxy: {PROXY_PROXIES['http']}")

# Keywords used to pre-select likely personal candidates before sending them to the LLM.
PERSONAL_KEYWORDS = [
    ("личный", 2),
    ("приватный", 2),
    ("индивидуальный", 2),
    ("персональный", 2),
    ("не общий", 2),
    ("один владелец", 2),
    ("one owner", 2),
    ("personal", 2),
    ("private", 2),
    ("not shared", 2),
    ("single device", 2),
    ("single", 1),
    ("один", 1),
    ("solo", 1),
]

SHARED_KEYWORDS = [
    ("общий", 2),
    ("shared", 2),
    ("general account", 2),
    ("общий доступ", 2),
    ("до 10 людей", 2),
    ("до n людей", 2),
    ("multiple", 1),
    ("several", 1),
    ("split", 1),
    ("group", 1),
    ("людей", 1),
    ("people", 1),
]


def _normalize_url(path: str) -> str:
    return urljoin(FUNPAY_BASE_URL, path)


def _extract_price(text: str) -> tuple[float, str]:
    """Extract numeric price and currency from a price string."""
    text = text.replace("\xa0", " ").replace(" ", "").replace(",", ".")
    match = re.search(r"([0-9]+(?:\.[0-9]+)?)", text)
    if not match:
        return 0.0, ""
    price = float(match.group(1))
    currency = re.sub(r"[0-9\s.]", "", text).strip()
    return price, currency


def _category_url(category_id: int, page: int = 1) -> str:
    """Build a Funpay category URL."""
    lang_part = f"{FUNPAY_LANG}/" if FUNPAY_LANG and FUNPAY_LANG != "ru" else ""
    base = f"{FUNPAY_BASE_URL}/{lang_part}lots/{category_id}/"
    params = {}
    if page > 1:
        params["page"] = page
    return f"{base}?{urlencode(params)}" if params else base


def _search_page_url(query: str, page: int = 1) -> str:
    """Build a Funpay search URL (fallback, may not work for all regions)."""
    lang_part = f"{FUNPAY_LANG}/" if FUNPAY_LANG and FUNPAY_LANG != "ru" else ""
    base = f"{FUNPAY_BASE_URL}/{lang_part}lots"
    params = {"query": query}
    if page > 1:
        params["page"] = page
    return f"{base}?{urlencode(params)}"


def _extract_listing_cards(soup: BeautifulSoup) -> list[BeautifulSoup]:
    """Funpay listing cards have the class 'tc-item'."""
    return soup.find_all("a", class_="tc-item")


def _parse_card(card: BeautifulSoup) -> Listing | None:
    """Parse one listing card into a Listing object."""
    url = _normalize_url(card.get("href", ""))
    lot_id = card.get("href", "")
    m = re.search(r"[?&]id=(\d+)", lot_id)
    lot_id = m.group(1) if m else re.sub(r"\D", "", lot_id)[:20] or "unknown"

    desc_el = card.select_one(".tc-desc")
    description = desc_el.get_text(" ", strip=True) if desc_el else ""

    price_text = ""
    price = 0.0
    currency = ""
    price_el = card.select_one(".tc-price")
    if price_el:
        price_text = price_el.get_text(strip=True)
        price, currency = _extract_price(price_text)

    seller = ""
    seller_el = card.select_one(".tc-user")
    if seller_el:
        seller = seller_el.get_text(" ", strip=True)

    subscription = card.get("data-f-subscription", "")
    plus_type = card.get("data-f-type", "")

    if not description or not url:
        return None

    listing = Listing(
        id=str(lot_id),
        title=description,
        description=description,
        price=price,
        currency=currency,
        seller=seller,
        url=url,
        raw_html=str(card),
    )
    listing.classification_reason = f"hint: subscription={subscription}, type={plus_type}"
    return listing


def _is_plus_hint(card: BeautifulSoup) -> bool:
    """Use Funpay data attributes to guess if a listing is ChatGPT Plus."""
    return (
        card.get("data-f-type") == "plus"
        or card.get("data-f-subscription") == "с подпиской"
    )


def _personal_score(text: str) -> int:
    """Return a heuristic score: higher means more likely a personal account."""
    t = text.lower()
    score = 0
    for keyword, weight in PERSONAL_KEYWORDS:
        if keyword in t:
            score += weight
    for keyword, weight in SHARED_KEYWORDS:
        if keyword in t:
            score -= weight
    return score


def select_personal_candidates(
    listings: list[Listing], max_candidates: int = 40, stop_event=None
) -> list[Listing]:
    """
    Pre-filter Plus listings to those that look like personal accounts based on keywords.
    Sorts by heuristic score then by price.  The final personal/shared decision is still
    made by the LLM; this just reduces API cost.
    """
    if stop_event and stop_event.is_set():
        return []

    scored = []
    for listing in listings:
        if stop_event and stop_event.is_set():
            break
        score = _personal_score(listing.description)
        # Keep only items with a positive personal signal, or at least no strong shared signal.
        if score >= 0:
            scored.append((score, listing.price, listing))

    scored.sort(key=lambda x: (-x[0], x[1]))
    return [listing for _, _, listing in scored[:max_candidates]]


def fetch_category_listings(
    category_id: int,
    max_pages: int | None = None,
    only_plus: bool = True,
    stop_event=None,
) -> list[Listing]:
    """Fetch all listings for a Funpay category."""
    max_pages = max_pages or MAX_PAGES
    results: list[Listing] = []
    seen_ids: set[str] = set()

    for page in range(1, max_pages + 1):
        if stop_event and stop_event.is_set():
            return results

        url = _category_url(category_id, page)
        try:
            resp = _SESSION.get(url, timeout=30)
            resp.raise_for_status()
        except requests.RequestException as exc:
            print(f"[scraper] Failed to fetch page {page}: {exc}")
            break

        soup = BeautifulSoup(resp.text, "lxml")
        cards = _extract_listing_cards(soup)

        if not cards:
            print(f"[scraper] No listing cards found on page {page} (url: {url})")
            break

        for card in cards:
            if stop_event and stop_event.is_set():
                return results
            listing = _parse_card(card)
            if not listing or listing.id in seen_ids:
                continue
            seen_ids.add(listing.id)
            if only_plus and not _is_plus_hint(card):
                continue
            results.append(listing)

        time.sleep(1.0)

    return results


def search_listings(query: str = "chatgpt plus", max_pages: int | None = None, stop_event=None) -> list[Listing]:
    """Search Funpay by query. Falls back to category 1355 (ChatGPT) if search fails."""
    max_pages = max_pages or MAX_PAGES
    results: list[Listing] = []
    seen_ids: set[str] = set()

    for page in range(1, max_pages + 1):
        if stop_event and stop_event.is_set():
            return results

        url = _search_page_url(query, page)
        try:
            resp = _SESSION.get(url, timeout=20)
            resp.raise_for_status()
        except requests.RequestException as exc:
            print(f"[scraper] Search failed: {exc}. Switching to ChatGPT category.")
            return fetch_category_listings(1355, max_pages=max_pages, stop_event=stop_event)

        soup = BeautifulSoup(resp.text, "lxml")
        cards = _extract_listing_cards(soup)

        if not cards:
            print(f"[scraper] No listing cards found on search page {page}. Switching to ChatGPT category.")
            return fetch_category_listings(1355, max_pages=max_pages, stop_event=stop_event)

        for card in cards:
            if stop_event and stop_event.is_set():
                return results
            listing = _parse_card(card)
            if listing and listing.id not in seen_ids:
                seen_ids.add(listing.id)
                results.append(listing)

        time.sleep(1.2)

    return results


def fetch_listing_description(listing: Listing, stop_event=None) -> str:
    """Fetch the lot page and extract its full description if needed."""
    if stop_event and stop_event.is_set():
        return listing.description

    try:
        resp = _SESSION.get(listing.url, timeout=20)
        resp.raise_for_status()
    except requests.RequestException as exc:
        print(f"[scraper] Failed to fetch lot {listing.id}: {exc}")
        return listing.description

    soup = BeautifulSoup(resp.text, "lxml")
    for desc_sel in [".tc-desc-text", ".lot-desc", ".description", "[class*='desc']"]:
        el = soup.select_one(desc_sel)
        if el:
            return el.get_text(" ", strip=True)
    return listing.description
