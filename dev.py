"""Development runner: keeps app.py alive in the background and restarts it on source file changes."""
import os
import socket
import subprocess
import sys
import time
from pathlib import Path

# Line-buffer stdout/stderr so dev-runner messages appear immediately in the log file.
try:
    sys.stdout.reconfigure(line_buffering=True)
    sys.stderr.reconfigure(line_buffering=True)
except Exception:
    pass

from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

# Files and directories that should NOT trigger a restart.
IGNORE_DIRS = {'.venv', '.git', '__pycache__', 'node_modules', '.zig-cache', 'zig-out', 'tmp', 'instance'}
IGNORE_PREFIXES = ('.',)
IGNORE_SUFFIXES = ('.pyc', '.pyo', '.log', '.tmp', '.swp', '~', '.pid', '.json', '.db', '.sqlite', '.db-journal', '.db-wal', '.db-shm')

PORT = 5000
TARGET = [sys.executable, 'app.py']
ENV = os.environ.copy()
ENV['DEV_RUNNER'] = '1'
PID_FILE = Path('.dev-runner.pid')


def should_restart(path: str) -> bool:
    """Return True if a change to this path should restart the server."""
    p = Path(path)
    parts = set(p.parts)
    if parts & IGNORE_DIRS:
        return False
    if p.name.startswith(IGNORE_PREFIXES):
        return False
    if p.name.endswith(IGNORE_SUFFIXES):
        return False
    return True


def wait_for_port_free(port: int, timeout: float = 10.0) -> bool:
    """Wait until the local port is free or timeout expires."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                # Bind to 127.0.0.1 so we don't occupy the wildcard address.
                s.settimeout(1)
                s.bind(('127.0.0.1', port))
                return True
        except OSError:
            time.sleep(0.3)
    return False


class Reloader(FileSystemEventHandler):
    def __init__(self, runner: 'Runner'):
        self.runner = runner
        self.last_event = 0.0

    def on_any_event(self, event):
        if event.event_type in ('opened', 'closed', 'closed_no_write'):
            return
        if not should_restart(event.src_path):
            return
        now = time.time()
        if now - self.last_event < 0.5:
            return
        self.last_event = now
        print(f'[dev] {event.event_type}: {event.src_path}')
        self.runner.restart()


class Runner:
    def __init__(self):
        self.process = None
        self._restart_pending = False

    def start(self, retries: int = 3):
        print(f'[dev] Starting: {" ".join(TARGET)}')
        if not wait_for_port_free(PORT, timeout=10.0):
            print(f'[dev] Warning: port {PORT} may still be in use, starting anyway')
        for attempt in range(1, retries + 1):
            self.process = subprocess.Popen(TARGET, cwd=os.getcwd(), env=ENV)
            time.sleep(0.5)
            if self.process.poll() is None:
                return
            print(f'[dev] app.py exited immediately (attempt {attempt}), retrying...')
            time.sleep(1.5)
        print('[dev] Failed to start app.py after multiple attempts')

    def stop(self):
        if self.process is None or self.process.poll() is not None:
            return
        print(f'[dev] Stopping process {self.process.pid}')
        self.process.terminate()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            print('[dev] Process did not terminate, killing...')
            self.process.kill()
            self.process.wait()
        # Give the OS a moment to release the port.
        wait_for_port_free(PORT, timeout=2.0)

    def restart(self):
        if self._restart_pending:
            return
        self._restart_pending = True
        print('[dev] Restarting due to file change...')
        self.stop()
        # On macOS the old socket may linger in TIME_WAIT; give it a moment.
        time.sleep(1.0)
        self.start()
        self._restart_pending = False

    def run(self):
        self.start()
        try:
            while True:
                time.sleep(1)
                if self.process.poll() is not None:
                    print('[dev] app.py exited unexpectedly, restarting...')
                    time.sleep(1)
                    self.start()
        except KeyboardInterrupt:
            print('\n[dev] Interrupted by user')
        finally:
            self.stop()


def _is_running(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except (OSError, ProcessLookupError):
        return False


def _acquire_lock() -> bool:
    if PID_FILE.exists():
        try:
            old_pid = int(PID_FILE.read_text().strip())
            if _is_running(old_pid):
                print(f'[dev] Another dev runner is already running (PID {old_pid}). Exiting.')
                return False
        except ValueError:
            pass
    PID_FILE.write_text(str(os.getpid()))
    return True


def _release_lock():
    try:
        PID_FILE.unlink(missing_ok=True)
    except Exception:
        pass


def main():
    if not _acquire_lock():
        sys.exit(1)

    root = os.getcwd()
    print(f'[dev] Watching: {root}')
    print(f'[dev] Server: http://127.0.0.1:{PORT}')
    print('[dev] Press Ctrl+C to stop')

    runner = Runner()
    handler = Reloader(runner)
    observer = Observer()
    observer.schedule(handler, root, recursive=True)
    observer.start()

    try:
        runner.run()
    finally:
        observer.stop()
        observer.join()
        _release_lock()


if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        _release_lock()
        sys.exit(0)
