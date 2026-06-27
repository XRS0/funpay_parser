"""Entry point and reusable runner for the Funpay parser."""
import argparse
import json
import threading
from typing import Callable, Iterable

from classifier import classify_listings
from duration import duration_matches, extract_target_duration
from models import Listing
from scraper import (
    fetch_category_listings,
    fetch_listing_description,
    search_listings,
    select_personal_candidates,
)


class ParserCancelled(Exception):
    """Raised when the user requests to stop the parser."""


def _listing_text(listing: Listing) -> str:
    """Return the combined text used for duration matching."""
    return f"{listing.title or ''} {listing.description or ''}"


def find_cheapest_personal(
    listings: Iterable[Listing],
    target_duration_days: float | None = None,
) -> Listing | None:
    """Return the cheapest personal ChatGPT Plus listing matching the requested duration.

    If a target duration is specified (e.g. 30 days), listings with a conflicting
    duration (e.g. 1 hour) are excluded. Listings that explicitly match the target
    are preferred; if none are found, listings without duration information are used
    as a fallback.
    """
    personal = [
        l for l in listings
        if l.is_plus and l.account_type == "personal" and l.price > 0
    ]
    if not personal:
        return None

    if target_duration_days is None:
        return min(personal, key=lambda l: l.price)

    matching = [
        l for l in personal
        if duration_matches(_listing_text(l), target_duration_days, allow_unknown=False)
    ]
    if matching:
        return min(matching, key=lambda l: l.price)

    fallback = [
        l for l in personal
        if duration_matches(_listing_text(l), target_duration_days, allow_unknown=True)
    ]
    if fallback:
        return min(fallback, key=lambda l: l.price)

    return None


def _noop(status: str) -> None:
    """Default progress callback that does nothing."""
    pass


def run_parser(
    category_id: int = 1355,
    query: str = "chatgpt plus",
    use_search: bool = False,
    pages: int | None = None,
    candidates: int = 40,
    deep: bool = False,
    output: str = "results.json",
    progress: Callable[[str], None] = _noop,
    stop_event: threading.Event | None = None,
) -> dict:
    """Run the parser and return a structured result dict."""
    target_duration = extract_target_duration(query)

    if use_search:
        progress(f"🔍 Searching Funpay for '{query}'...")
        listings = search_listings(query=query, max_pages=pages, stop_event=stop_event)
    else:
        progress(f"🔍 Fetching ChatGPT category (ID={category_id})...")
        listings = fetch_category_listings(
            category_id=category_id, max_pages=pages, stop_event=stop_event
        )

    if stop_event and stop_event.is_set():
        raise ParserCancelled("Stopped by user")

    progress(f"📦 Found {len(listings)} ChatGPT Plus listings")
    if target_duration:
        progress(f"⏱️ Duration filter: {target_duration:.0f} days from query")

    if not listings:
        progress("No listings found. Try changing the query/category or check if Funpay blocks the request.")
        return {"success": False, "error": "No listings found", "listings": [], "cheapest": None}

    selected = select_personal_candidates(listings, max_candidates=candidates, stop_event=stop_event)
    if stop_event and stop_event.is_set():
        raise ParserCancelled("Stopped by user")

    progress(f"🧠 Pre-selected {len(selected)} likely-personal candidates for LLM verification...")

    if not selected:
        progress("No candidates found. Try increasing --pages or --candidates.")
        return {"success": False, "error": "No candidates found", "listings": [], "cheapest": None}

    if deep:
        progress("🌐 Fetching full descriptions for each candidate...")
        for listing in selected:
            if stop_event and stop_event.is_set():
                raise ParserCancelled("Stopped by user")
            listing.description = fetch_listing_description(listing, stop_event=stop_event)

    if stop_event and stop_event.is_set():
        raise ParserCancelled("Stopped by user")

    progress("🧠 Classifying candidates with LLM...")
    classified = classify_listings(selected, stop_event=stop_event)
    if stop_event and stop_event.is_set():
        raise ParserCancelled("Stopped by user")

    all_results = classified + [l for l in listings if l not in selected]

    cheapest = find_cheapest_personal(classified, target_duration)
    personal = [l for l in classified if l.is_plus and l.account_type == "personal"]
    shared = [l for l in classified if l.is_plus and l.account_type == "shared"]
    other = [l for l in classified if l.is_plus and l.account_type not in ("personal", "shared")]

    summary = {
        "total_plus": len(listings),
        "classified": len(classified),
        "personal": len(personal),
        "shared": len(shared),
        "other": len(other),
    }

    if cheapest:
        progress("✅ Cheapest personal ChatGPT Plus account found!")
    else:
        progress("❌ No personal ChatGPT Plus account confirmed among the candidates.")

    with open(output, "w", encoding="utf-8") as f:
        json.dump([l.to_dict() for l in all_results], f, ensure_ascii=False, indent=2)
    progress(f"💾 Full results saved to {output}")

    return {
        "success": True,
        "summary": summary,
        "cheapest": cheapest.to_dict() if cheapest else None,
        "all_results": [l.to_dict() for l in all_results],
    }


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Find the cheapest personal ChatGPT Plus account on Funpay."
    )
    parser.add_argument(
        "--category-id",
        type=int,
        default=1355,
        help="Funpay category ID for ChatGPT (default: 1355).",
    )
    parser.add_argument(
        "--query",
        default="chatgpt plus",
        help="Search query for Funpay (used only if --use-search is set).",
    )
    parser.add_argument(
        "--use-search",
        action="store_true",
        help="Use search instead of category ID.",
    )
    parser.add_argument(
        "--pages", type=int, default=None, help="Number of pages to scan."
    )
    parser.add_argument(
        "--candidates",
        type=int,
        default=40,
        help="How many likely-personal candidates to send to the LLM.",
    )
    parser.add_argument(
        "--deep",
        action="store_true",
        help="Fetch full description for each lot before classification.",
    )
    parser.add_argument(
        "--output", default="results.json", help="Path to save the full results."
    )
    args = parser.parse_args()

    result = run_parser(
        category_id=args.category_id,
        query=args.query,
        use_search=args.use_search,
        pages=args.pages,
        candidates=args.candidates,
        deep=args.deep,
        output=args.output,
        progress=print,
    )

    if result["success"] and result["cheapest"]:
        c = result["cheapest"]
        print("\n✅ Cheapest personal ChatGPT Plus account:")
        print(f"   Title:       {c['title']}")
        print(f"   Price:       {c['price']} {c['currency']}")
        print(f"   Seller:      {c['seller']}")
        print(f"   URL:         {c['url']}")
        print(f"   Confidence:  {c['confidence']:.2f}")
        print(f"   Reason:      {c['classification_reason']}")
    elif result["success"]:
        print("\n❌ No personal ChatGPT Plus account confirmed among the candidates.")
        print("   Tip: increase --candidates or --pages to scan more listings.")


if __name__ == "__main__":
    main()
