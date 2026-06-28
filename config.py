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
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY")
OPENROUTER_MODEL = os.getenv(
    "OPENROUTER_MODEL",
    "openai/gpt-4o-mini",
)
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "fireworks")
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


def _migrate_settings() -> None:
    """Migrate legacy Fireworks-only settings to generic LLM settings."""
    data = _load_settings()
    if "fireworks_api_key" in data and "llm_api_key" not in data:
        data["llm_provider"] = "fireworks"
        data["llm_api_key"] = data.pop("fireworks_api_key")
        if "llm_model" not in data:
            data["llm_model"] = FIREWORKS_MODEL
        _save_settings(data)


def get_llm_provider() -> str:
    """Return the active LLM provider."""
    settings = _load_settings()
    provider = settings.get("llm_provider")
    if provider in ("fireworks", "openrouter"):
        return provider
    env_provider = os.getenv("LLM_PROVIDER")
    if env_provider in ("fireworks", "openrouter"):
        return env_provider
    return "fireworks"


def set_llm_provider(provider: str) -> None:
    """Persist the LLM provider."""
    data = _load_settings()
    data["llm_provider"] = provider
    _save_settings(data)


def get_llm_api_key() -> str | None:
    """Return the API key for the active provider."""
    settings = _load_settings()
    key = settings.get("llm_api_key")
    if key:
        return key
    provider = get_llm_provider()
    if provider == "openrouter":
        return os.getenv("OPENROUTER_API_KEY") or None
    return os.getenv("FIREWORKS_API_KEY") or None


def set_llm_api_key(key: str) -> None:
    """Persist the API key for the active provider."""
    data = _load_settings()
    data["llm_api_key"] = key
    _save_settings(data)


def get_llm_model() -> str:
    """Return the model for the active provider."""
    settings = _load_settings()
    model = settings.get("llm_model")
    if model:
        return model
    provider = get_llm_provider()
    if provider == "openrouter":
        return os.getenv("OPENROUTER_MODEL", OPENROUTER_MODEL)
    return os.getenv("FIREWORKS_MODEL", FIREWORKS_MODEL)


def set_llm_model(model: str) -> None:
    """Persist the model for the active provider."""
    data = _load_settings()
    data["llm_model"] = model
    _save_settings(data)


# Legacy helpers kept for compatibility.
def get_fireworks_api_key() -> str | None:
    """Return the Fireworks API key if Fireworks is the active provider."""
    if get_llm_provider() == "fireworks":
        return get_llm_api_key()
    return None


def set_fireworks_api_key(key: str) -> None:
    """Persist the API key when using Fireworks."""
    if get_llm_provider() == "fireworks":
        set_llm_api_key(key)
    else:
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


_migrate_settings()
