"""Duration extraction and matching for listings and queries."""
import re
from typing import Iterable

# Regex -> multiplier to convert the matched unit to days.
_DURATION_PATTERNS = [
    (re.compile(r"(\d+)\s*(?:час|часа|часов|ч|hour|hours|hr|hrs|h)\b", re.IGNORECASE), 1 / 24),
    (re.compile(r"(\d+)\s*(?:день|дня|дней|д|day|days|d)\b", re.IGNORECASE), 1),
    (re.compile(r"(\d+)\s*(?:месяц|месяца|месяцев|мес|month|months|mo|m)\b", re.IGNORECASE), 30),
    (re.compile(r"(\d+)\s*(?:год|года|лет|year|years|yr|y)\b", re.IGNORECASE), 365),
    (re.compile(r"(?:полгода|half.year|halfyear)", re.IGNORECASE), 180),
]

# Approximate tolerance for matching a target duration.
_TOLERANCE_FACTOR = 0.35


def extract_durations(text: str | None) -> list[float]:
    """Return all durations found in text, expressed in days."""
    if not text:
        return []
    values = []
    for pattern, multiplier in _DURATION_PATTERNS:
        for match in pattern.finditer(text):
            if match.lastindex:
                try:
                    values.append(float(match.group(1)) * multiplier)
                except ValueError:
                    pass
            else:
                # Fixed phrases like "полгода" without a capture group.
                values.append(multiplier)
    return values


def extract_target_duration(query: str | None) -> float | None:
    """Try to extract a target duration (in days) from the user's query."""
    if not query:
        return None
    durations = extract_durations(query)
    # A query is assumed to contain at most one meaningful duration.
    return durations[0] if durations else None


def _matches_target(listing_durations: Iterable[float], target: float) -> bool:
    """Return True if any duration in the listing matches the target."""
    tolerance = target * _TOLERANCE_FACTOR
    return any(abs(d - target) <= tolerance for d in listing_durations)


def _conflicts_with_target(listing_durations: Iterable[float], target: float) -> bool:
    """Return True if the listing has a duration that is clearly different from the target.

    We treat a conflict as a duration that is outside the tolerance window and not
    approximately a multiple of the target (e.g. 1 year when the target is 1 month is fine).
    """
    tolerance = target * _TOLERANCE_FACTOR
    for d in listing_durations:
        if abs(d - target) <= tolerance:
            continue
        # If the listing duration is a multiple of the target, it still matches the intent.
        if d >= target * 2 and abs(d - round(d / target) * target) <= tolerance:
            continue
        return True
    return False


def duration_matches(
    text: str | None,
    target_days: float | None,
    *,
    allow_unknown: bool = True,
) -> bool:
    """Return True if the listing text matches the target duration.

    - If no target is specified, everything matches.
    - If the listing has no duration info, it matches only when allow_unknown is True.
    - If the listing has a duration matching the target (within tolerance), it matches.
    - If the listing has a conflicting duration (e.g. 1 hour when target is 30 days), it does not match.
    """
    if target_days is None:
        return True

    durations = extract_durations(text)
    if not durations:
        return allow_unknown

    if _matches_target(durations, target_days):
        return True

    if _conflicts_with_target(durations, target_days):
        return False

    return allow_unknown
