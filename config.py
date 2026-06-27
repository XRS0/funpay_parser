"""Project configuration loaded from environment and persisted settings."""
import json
import os
from pathlib import Path

from dotenv import load_dotenv

load_dotenv(".env")

DATA_DIR = os.getenv("DATA_DIR", ".")
Path(DATA_DIR).mkdir(parents=True, exist_ok=True)

SETTINGS_FILE = Path(DATA_DIR) / "settings.json"
RUN_STATE_FILE = Path(DATA_DIR) / "run_state.json"

FIREWORKS_API_KEY = os.getenv("FIREWORKS_API_KEY")
FIREWORKS_MODEL = os.getenv(
    "FIREWORKS_MODEL",
    "accounts/fireworks/models/llama-v3p1-70b-instruct",
)
FUNPAY_BASE_URL = os.getenv("FUNPAY_BASE_URL", "https://funpay.com").rstrip("/")
FUNPAY_LANG = os.getenv("FUNPAY_LANG", "en")
MAX_PAGES = int(os.getenv("MAX_PAGES", "3"))
PROXY = os.getenv("PROXY", "")


def parse_proxy(proxy_string: str) -> dict[str, str] | None:
    """Convert 'host:port@user:pass' into a requests proxies dict."""
    if not proxy_string or "@" not in proxy_string or ":" not in proxy_string:
        return None
    try:
        host_port, credentials = proxy_string.split("@", 1)
        user, password = credentials.split(":", 1)
        host, port = host_port.rsplit(":", 1)
        # proxy.market proxies are SOCKS5 by default.
        url = f"socks5://{user}:{password}@{host}:{port}"
        return {"http": url, "https": url}
    except ValueError:
        return None


PROXY_PROXIES = parse_proxy(PROXY)


def _load_settings() -> dict:
    """Read persisted settings from JSON, returning an empty dict on failure."""
    try:
        if SETTINGS_FILE.exists():
            return json.loads(SETTINGS_FILE.read_text(encoding="utf-8"))
    except Exception:
        pass
    return {}


def _save_settings(data: dict) -> None:
    """Persist settings to JSON."""
    SETTINGS_FILE.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def get_fireworks_api_key() -> str | None:
    """Return the Fireworks API key from persisted settings or env var fallback."""
    settings = _load_settings()
    key = settings.get("fireworks_api_key")
    if key:
        return key
    return os.getenv("FIREWORKS_API_KEY") or None


def set_fireworks_api_key(key: str) -> None:
    """Persist the Fireworks API key to settings."""
    data = _load_settings()
    data["fireworks_api_key"] = key
    _save_settings(data)


def get_setting(key: str, default: str | None = None) -> str | None:
    """Return an arbitrary persisted setting."""
    return _load_settings().get(key, default)


def set_setting(key: str, value: str) -> None:
    """Persist an arbitrary setting."""
    data = _load_settings()
    data[key] = value
    _save_settings(data)
