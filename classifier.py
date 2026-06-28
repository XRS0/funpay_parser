"""Classify Funpay listings using a configurable LLM provider (Fireworks or OpenRouter)."""
import json
import time
from typing import Iterable

import requests

from config import get_llm_api_key, get_llm_model, get_llm_provider
from models import Listing


def _build_prompt(listing: Listing) -> str:
    return (
        "You are analyzing a Funpay marketplace listing for a ChatGPT account.\n\n"
        f"Title: {listing.title}\n"
        f"Description: {listing.description}\n"
        f"Price: {listing.price} {listing.currency}\n\n"
        "Answer the following questions:\n"
        "1. Is this listing for a ChatGPT Plus subscription/account? (yes/no)\n"
        "2. Is the account 'personal' (private, single-owner, individual access) "
        "or 'shared' (multi-user, family/shared access, account splitting)? "
        "Answer exactly one of: personal, shared, unknown.\n"
        "3. Confidence in your classification from 0.0 to 1.0.\n"
        "4. Short reason (one sentence).\n\n"
        "Respond ONLY with valid JSON in this exact format:\n"
        '{"is_plus": true|false, "account_type": "personal"|"shared"|"unknown", '
        '"confidence": 0.0-1.0, "reason": "..."}'
    )


def _provider_config() -> tuple[str, str, str, dict]:
    """Return (provider, url, model, headers) for the active LLM provider."""
    provider = get_llm_provider()
    api_key = get_llm_api_key()
    if not api_key:
        raise ValueError(f"{provider.capitalize()} API key is not configured. Set it in Settings.")
    model = get_llm_model()

    if provider == "openrouter":
        url = "https://openrouter.ai/api/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "HTTP-Referer": "https://github.com/XRS0/funpay_parser",
            "X-Title": "Funpay Parser",
        }
    else:
        url = "https://api.fireworks.ai/inference/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }

    return provider, url, model, headers


def classify_listing(listing: Listing, stop_event=None) -> dict:
    """Classify a single listing using the active LLM provider."""
    provider, url, model, headers = _provider_config()

    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": "You are a helpful classifier that outputs only JSON."},
            {"role": "user", "content": _build_prompt(listing)},
        ],
        "temperature": 0.1,
        "max_tokens": 256,
        "response_format": {"type": "json_object"},
    }

    resp = requests.post(url, headers=headers, json=payload, timeout=60)
    resp.raise_for_status()
    data = resp.json()

    content = data["choices"][0]["message"]["content"]
    parsed = json.loads(content)

    return {
        "is_plus": bool(parsed.get("is_plus", False)),
        "account_type": parsed.get("account_type", "unknown").lower().strip(),
        "confidence": float(parsed.get("confidence", 0.0)),
        "reason": parsed.get("reason", ""),
    }


def classify_listings(listings: Iterable[Listing], delay: float = 0.5, stop_event=None) -> list[Listing]:
    """Classify multiple listings, applying results in-place."""
    classified: list[Listing] = []
    for listing in listings:
        if stop_event and stop_event.is_set():
            return classified
        try:
            result = classify_listing(listing)
            listing.is_plus = result["is_plus"]
            listing.account_type = result["account_type"]
            listing.confidence = result["confidence"]
            listing.classification_reason = result["reason"]
            classified.append(listing)
            print(
                f"[classifier] {listing.title[:50]:50} | "
                f"plus={listing.is_plus}, type={listing.account_type}, "
                f"conf={listing.confidence:.2f}"
            )
        except Exception as exc:
            listing.account_type = "unknown"
            listing.confidence = 0.0
            listing.classification_reason = f"classification error: {exc}"
            classified.append(listing)
            print(f"[classifier] Failed to classify lot {listing.id}: {exc}")
        time.sleep(delay)
    return classified
