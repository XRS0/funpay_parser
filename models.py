"""Data models for the parser."""
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Listing:
    """Represents one Funpay lot."""

    id: str
    title: str
    description: str
    price: float
    currency: str
    seller: str
    url: str
    raw_html: str = field(repr=False, default="")
    is_plus: Optional[bool] = None
    account_type: Optional[str] = None  # "personal", "shared", "unknown"
    confidence: Optional[float] = None
    classification_reason: str = ""

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "title": self.title,
            "description": self.description,
            "price": self.price,
            "currency": self.currency,
            "seller": self.seller,
            "url": self.url,
            "is_plus": self.is_plus,
            "account_type": self.account_type,
            "confidence": self.confidence,
            "classification_reason": self.classification_reason,
        }
