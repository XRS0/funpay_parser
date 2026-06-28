"""Web UI for the Funpay ChatGPT Plus parser with profiles and saved results."""
import atexit
import json
import os
import threading
import time
from pathlib import Path

from flask import Flask, jsonify, render_template, request

from database import (
    create_profile,
    create_schedule,
    db,
    delete_profile,
    delete_saved_result,
    delete_schedule,
    get_profile,
    get_saved_result,
    get_schedule,
    init_db,
    list_profiles,
    list_saved_results,
    list_schedules,
    save_result,
    update_profile,
    update_schedule,
)
from main import ParserCancelled, run_parser
from scheduler import ParserScheduler
from config import (
    get_llm_api_key,
    get_llm_model,
    get_llm_provider,
    set_llm_api_key,
    set_llm_model,
    set_llm_provider,
    RUN_STATE_FILE,
)

app = Flask(__name__)
_db_path = os.getenv("DATABASE_PATH", "parser.db")
if not _db_path.startswith("sqlite:///"):
    _db_path = f"sqlite:///{_db_path}"
app.config["SQLALCHEMY_DATABASE_URI"] = _db_path
app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = False


def _static_mtime(filename: str) -> int:
    try:
        return int(os.path.getmtime(os.path.join(app.static_folder, filename)))
    except OSError:
        return 0


@app.url_defaults
def _add_static_version(endpoint: str, values: dict) -> None:
    """Append a file-mtime query param to static URLs for cache busting."""
    if endpoint == "static" and "filename" in values and "v" not in values:
        values["v"] = _static_mtime(values["filename"])


@app.after_request
def _add_cache_headers(response):
    """Cache static files for a long time since URLs are versioned."""
    if request.path.startswith("/static/"):
        response.headers["Cache-Control"] = "public, max-age=31536000, immutable"
    return response


init_db(app)

# In-memory state for the background parser task.
_state = {
    "running": False,
    "status": "idle",
    "progress": [],
    "result": None,
    "error": None,
    "started_at": None,
    "finished_at": None,
    "profile_id": None,
}

_lock = threading.Lock()

# Stop event for the currently running parser task (user or scheduled).
_stop_event = None


def _persist_state() -> None:
    """Write the current run state to disk so the UI survives page reloads."""
    try:
        RUN_STATE_FILE.write_text(json.dumps(_state, ensure_ascii=False, default=str), encoding="utf-8")
    except Exception:
        pass


def _clear_persisted_state() -> None:
    """Remove the saved run state so finished runs don't persist across reloads."""
    try:
        if RUN_STATE_FILE.exists():
            RUN_STATE_FILE.unlink()
    except Exception:
        pass


def _load_state() -> None:
    """Restore the last run state from disk.

    Only in-progress runs are restored. Finished runs are intentionally not
    restored across reloads, so the stale file is removed.

    If a run was marked as running when the file was written, it means the
    server process was restarted and the background thread is gone, so we
    mark it as interrupted.
    """
    global _state
    try:
        if RUN_STATE_FILE.exists():
            saved = json.loads(RUN_STATE_FILE.read_text(encoding="utf-8"))
            if saved.get("finished_at"):
                RUN_STATE_FILE.unlink()
                return
            if saved.get("running"):
                saved["running"] = False
                saved["status"] = "Interrupted"
                saved["error"] = saved.get("error") or "Server restarted while the parser was running."
                saved["finished_at"] = time.strftime("%H:%M:%S")
            _state = saved
            _persist_state()
    except Exception:
        pass


def _set_status(message: str) -> None:
    """Callback passed to run_parser; records progress lines and updates the current status."""
    with _lock:
        _state["status"] = message
        _state["progress"].append({"time": time.strftime("%H:%M:%S"), "message": message})
        _persist_state()


def _append_progress(message: str) -> None:
    """Append a progress line without changing the top-level status."""
    with _lock:
        _state["progress"].append({"time": time.strftime("%H:%M:%S"), "message": message})
        _persist_state()


def _run_task(profile_id: int | None, options: dict, stop_event: threading.Event) -> None:
    """Background worker that runs the parser and optionally saves the result."""
    global _stop_event
    _stop_event = stop_event
    with _lock:
        _state["status"] = "Starting parser..."
        _state["progress"] = []
        _state["result"] = None
        _state["error"] = None
        _state["started_at"] = time.strftime("%H:%M:%S")
        _state["finished_at"] = None
        _state["profile_id"] = profile_id
        _persist_state()

    try:
        result = run_parser(
            category_id=options.get("category_id", 1355),
            query=options.get("query", "chatgpt plus"),
            use_search=False,
            pages=options.get("pages") or options.get("max_pages") or None,
            candidates=options.get("candidates", 40),
            deep=options.get("deep", False),
            output="results.json",
            progress=_set_status,
            stop_event=stop_event,
        )
        with _lock:
            _state["result"] = result
            _state["status"] = "Done" if result.get("success") else "Failed"

        if result.get("success") and profile_id:
            try:
                with app.app_context():
                    save_result(
                        profile_id=profile_id,
                        cheapest=result.get("cheapest"),
                        summary=result.get("summary"),
                        all_results=result.get("all_results", []),
                    )
                _append_progress("💾 Result saved to database")
            except Exception as exc:
                _append_progress(f"⚠️ Could not save result: {exc}")
            finally:
                with _lock:
                    _state["status"] = "Done"

    except ParserCancelled:
        _set_status("⏹️ Обработка остановлена пользователем")
        with _lock:
            _state["status"] = "Остановлено"

    except Exception as exc:
        _set_status(f"Error: {exc}")
        with _lock:
            _state["error"] = str(exc)
            _state["status"] = f"Error: {exc}"
    finally:
        with _lock:
            _state["running"] = False
            _stop_event = None
            _state["finished_at"] = time.strftime("%H:%M:%S")
            _clear_persisted_state()


def _run_scheduled(profile_id: int, options: dict) -> None:
    """Scheduler callback that starts a parser run if no other run is active."""
    with _lock:
        if _state["running"]:
            _append_progress("⏰ Scheduled run skipped: another run is already in progress")
            return
        _state["running"] = True
        stop_event = threading.Event()
    _run_task(profile_id, options, stop_event)


parser_scheduler = ParserScheduler(app, _run_scheduled)


@app.route("/")
def index():
    return render_template("index.html")


@app.route("/saved")
def saved():
    return render_template("saved.html")


@app.route("/scheduler")
def scheduler_page():
    return render_template("scheduler.html")


@app.route("/settings")
def settings_page():
    return render_template("settings.html")


@app.route("/run", methods=["POST"])
def run():
    global _stop_event
    data = request.get_json() or {}
    profile_id = data.get("profile_id")

    if profile_id:
        profile = get_profile(int(profile_id))
        if not profile:
            return jsonify({"error": "Profile not found"}), 404
        options = profile.to_dict()
    else:
        options = {
            "category_id": int(data.get("category_id", 1355) or 1355),
            "query": data.get("query", "chatgpt plus"),
            "candidates": int(data.get("candidates", 40) or 40),
            "pages": int(data.get("pages", 0) or 0) or None,
            "deep": bool(data.get("deep", False)),
        }

    with _lock:
        if _state["running"]:
            return jsonify({"error": "A parse task is already running."}), 409
        _state["running"] = True
        _stop_event = threading.Event()

    thread = threading.Thread(target=_run_task, args=(profile_id, options, _stop_event), daemon=True)
    thread.start()

    return jsonify({"success": True, "status": "started"})


@app.route("/status")
def status():
    with _lock:
        return jsonify({
            "running": _state["running"],
            "status": _state["status"],
            "progress": list(_state["progress"]),
            "started_at": _state["started_at"],
            "finished_at": _state["finished_at"],
            "error": _state["error"],
            "profile_id": _state["profile_id"],
            "result_summary": _state["result"].get("summary") if _state["result"] else None,
            "cheapest": _state["result"].get("cheapest") if _state["result"] else None,
        })


@app.route("/stop", methods=["POST"])
def stop():
    global _stop_event
    with _lock:
        if not _state["running"]:
            return jsonify({"error": "No parse task is running."}), 409
        if not _stop_event:
            return jsonify({"error": "Stop event is not available."}), 409
        _stop_event.set()
        _state["status"] = "Останавливаю..."
        _persist_state()
    return jsonify({"success": True, "status": "stopping"})


@app.route("/results")
def results():
    with _lock:
        if not _state["result"]:
            return jsonify({"error": "No results yet"}), 404
        return jsonify(_state["result"])


@app.route("/api/profiles", methods=["GET"])
def api_profiles_list():
    profiles = list_profiles()
    return jsonify([p.to_dict() for p in profiles])


@app.route("/api/profiles", methods=["POST"])
def api_profiles_create():
    data = request.get_json() or {}
    if not data.get("name"):
        return jsonify({"error": "Name is required"}), 400

    profile = create_profile(
        name=data["name"],
        query=data.get("query", "chatgpt plus"),
        category_id=int(data.get("category_id", 1355) or 1355),
        candidates=int(data.get("candidates", 40) or 40),
        deep=bool(data.get("deep", False)),
        max_pages=int(data.get("max_pages", 0) or 0) or None,
    )
    return jsonify(profile.to_dict()), 201


@app.route("/api/profiles/<int:profile_id>", methods=["GET"])
def api_profiles_get(profile_id: int):
    profile = get_profile(profile_id)
    if not profile:
        return jsonify({"error": "Profile not found"}), 404
    return jsonify(profile.to_dict())


@app.route("/api/profiles/<int:profile_id>", methods=["PUT"])
def api_profiles_update(profile_id: int):
    data = request.get_json() or {}
    allowed = {
        "name": data.get("name"),
        "query": data.get("query"),
        "category_id": int(data["category_id"]) if data.get("category_id") is not None else None,
        "candidates": int(data["candidates"]) if data.get("candidates") is not None else None,
        "deep": data.get("deep") if isinstance(data.get("deep"), bool) else None,
        "max_pages": int(data["max_pages"]) if data.get("max_pages") is not None else None,
    }
    updates = {k: v for k, v in allowed.items() if v is not None or (isinstance(v, bool) and v is False)}

    profile = update_profile(profile_id, **updates)
    if not profile:
        return jsonify({"error": "Profile not found"}), 404
    return jsonify(profile.to_dict())


@app.route("/api/profiles/<int:profile_id>", methods=["DELETE"])
def api_profiles_delete(profile_id: int):
    if delete_profile(profile_id):
        return jsonify({"success": True})
    return jsonify({"error": "Profile not found"}), 404


@app.route("/api/profiles/<int:profile_id>/run", methods=["POST"])
def api_profiles_run(profile_id: int):
    global _stop_event
    profile = get_profile(profile_id)
    if not profile:
        return jsonify({"error": "Profile not found"}), 404

    with _lock:
        if _state["running"]:
            return jsonify({"error": "A parse task is already running."}), 409
        _state["running"] = True
        _stop_event = threading.Event()

    thread = threading.Thread(target=_run_task, args=(profile_id, profile.to_dict(), _stop_event), daemon=True)
    thread.start()

    return jsonify({"success": True, "status": "started"})


@app.route("/api/saved_results", methods=["GET"])
def api_saved_results_list():
    profile_id = request.args.get("profile_id", type=int)
    results = list_saved_results(profile_id=profile_id)
    return jsonify([r.to_dict() for r in results])


@app.route("/api/saved_results", methods=["POST"])
def api_saved_results_create():
    data = request.get_json() or {}
    profile_id = data.get("profile_id")
    if not profile_id:
        return jsonify({"error": "profile_id is required"}), 400
    if not get_profile(int(profile_id)):
        return jsonify({"error": "Profile not found"}), 404

    result = save_result(
        profile_id=int(profile_id),
        cheapest=data.get("cheapest"),
        summary=data.get("summary"),
        all_results=data.get("all_results", []),
    )
    return jsonify(result.to_dict()), 201


@app.route("/api/saved_results/<int:result_id>", methods=["GET"])
def api_saved_results_get(result_id: int):
    result = get_saved_result(result_id)
    if not result:
        return jsonify({"error": "Saved result not found"}), 404
    return jsonify(result.to_dict(include_results=True))


@app.route("/api/saved_results/<int:result_id>", methods=["DELETE"])
def api_saved_results_delete(result_id: int):
    if delete_saved_result(result_id):
        return jsonify({"success": True})
    return jsonify({"error": "Saved result not found"}), 404


@app.route("/api/schedules", methods=["GET"])
def api_schedules_list():
    schedules = list_schedules()
    profiles = {p.id: p for p in list_profiles()}
    data = []
    for s in schedules:
        item = s.to_dict()
        profile = profiles.get(s.profile_id)
        item["profile_name"] = profile.name if profile else f"Профиль #{s.profile_id}"
        data.append(item)
    return jsonify(data)


@app.route("/api/schedules", methods=["POST"])
def api_schedules_create():
    data = request.get_json() or {}
    profile_id = data.get("profile_id")
    interval = data.get("interval_minutes")
    if not profile_id:
        return jsonify({"error": "profile_id is required"}), 400
    if not interval or not isinstance(interval, int) or interval < 1:
        return jsonify({"error": "interval_minutes must be a positive integer"}), 400
    if not get_profile(int(profile_id)):
        return jsonify({"error": "Profile not found"}), 404

    schedule = create_schedule(
        profile_id=int(profile_id),
        interval_minutes=int(interval),
        enabled=bool(data.get("enabled", True)),
    )
    parser_scheduler.add_schedule(schedule)
    return jsonify(schedule.to_dict()), 201


@app.route("/api/schedules/<int:schedule_id>", methods=["PUT"])
def api_schedules_update(schedule_id: int):
    schedule = get_schedule(schedule_id)
    if not schedule:
        return jsonify({"error": "Schedule not found"}), 404

    data = request.get_json() or {}
    interval = data.get("interval_minutes")
    enabled = data.get("enabled")

    updates = {}
    if interval is not None and isinstance(interval, int) and interval >= 1:
        updates["interval_minutes"] = interval
    if isinstance(enabled, bool):
        updates["enabled"] = enabled

    if updates:
        schedule = update_schedule(schedule_id, **updates)
        parser_scheduler.update_schedule(schedule)
        schedule = get_schedule(schedule_id)
        db.session.refresh(schedule)
    return jsonify(schedule.to_dict())


@app.route("/api/schedules/<int:schedule_id>", methods=["DELETE"])
def api_schedules_delete(schedule_id: int):
    schedule = get_schedule(schedule_id)
    if not schedule:
        return jsonify({"error": "Schedule not found"}), 404
    parser_scheduler.remove_schedule(schedule_id)
    delete_schedule(schedule_id)
    return jsonify({"success": True})


@app.route("/api/schedules/<int:schedule_id>/run", methods=["POST"])
def api_schedules_run(schedule_id: int):
    global _stop_event
    schedule = get_schedule(schedule_id)
    if not schedule:
        return jsonify({"error": "Schedule not found"}), 404
    profile = get_profile(schedule.profile_id)
    if not profile:
        return jsonify({"error": "Profile not found"}), 404

    with _lock:
        if _state["running"]:
            return jsonify({"error": "A parse task is already running."}), 409
        _state["running"] = True
        _stop_event = threading.Event()

    thread = threading.Thread(target=_run_task, args=(profile.id, profile.to_dict(), _stop_event), daemon=True)
    thread.start()
    return jsonify({"success": True, "status": "started"})


@app.route("/api/settings", methods=["GET"])
def api_settings_get():
    """Return the current LLM provider, model, and masked API key."""
    provider = get_llm_provider()
    model = get_llm_model()
    key = get_llm_api_key() or ""
    masked = ""
    if key:
        masked = key[:4] + "*" * (len(key) - 4) if len(key) > 4 else "*" * len(key)
    return jsonify({
        "llm_provider": provider,
        "llm_model": model,
        "llm_api_key": masked,
        "has_key": bool(key),
        "default_models": {
            "fireworks": "accounts/fireworks/models/llama-v3p1-70b-instruct",
            "openrouter": "openai/gpt-4o-mini",
        },
    })


@app.route("/api/settings", methods=["PUT"])
def api_settings_update():
    """Persist LLM provider, model, and API key from the settings page."""
    data = request.get_json() or {}
    provider = data.get("llm_provider")
    if provider in ("fireworks", "openrouter"):
        set_llm_provider(provider)
    model = (data.get("llm_model") or "").strip()
    if "llm_model" in data:
        set_llm_model(model)
    key = (data.get("llm_api_key") or "").strip()
    if "llm_api_key" in data:
        set_llm_api_key(key)
    return jsonify({
        "success": True,
        "llm_provider": get_llm_provider(),
        "llm_model": get_llm_model(),
        "has_key": bool(get_llm_api_key()),
    })


@app.route("/files/results.json")
def serve_results_file():
    """Serve the generated results.json file directly."""
    path = Path("results.json")
    if not path.exists():
        return jsonify({"error": "results.json not found"}), 404
    return jsonify(path.read_text(encoding="utf-8"))


_load_state()

try:
    parser_scheduler.start()
except Exception:
    pass
atexit.register(lambda: parser_scheduler.shutdown())

if __name__ == "__main__":
    # When launched by dev.py, disable Flask's built-in reloader to avoid
    # double restarts and port conflicts; dev.py handles the watching itself.
    use_reloader = os.environ.get("DEV_RUNNER") != "1"
    app.run(host="0.0.0.0", port=5000, debug=True, threaded=True, use_reloader=use_reloader)
