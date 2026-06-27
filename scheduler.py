"""Background scheduler for automatic parser runs."""
from datetime import datetime, timezone
from typing import Callable

from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.triggers.interval import IntervalTrigger

from database import get_profile, get_schedule, list_schedules, update_schedule


class ParserScheduler:
    """Wraps APScheduler and keeps it in sync with the Schedule DB table."""

    def __init__(
        self,
        app,
        run_profile_func: Callable[[int, dict], None],
    ):
        self.app = app
        self.run_profile_func = run_profile_func
        self.scheduler = BackgroundScheduler()

    def start(self) -> None:
        """Start the scheduler and load jobs from the database."""
        if self.scheduler.running:
            return
        self.scheduler.start()
        self.sync_schedules()

    def shutdown(self) -> None:
        """Stop the scheduler."""
        if self.scheduler.running:
            self.scheduler.shutdown(wait=False)

    def sync_schedules(self) -> None:
        """Recreate scheduler jobs from the database."""
        with self.app.app_context():
            schedules = list_schedules()

        # Remove all existing jobs first to avoid duplicates.
        for job in list(self.scheduler.get_jobs()):
            try:
                self.scheduler.remove_job(job.id)
            except Exception:
                pass

        for schedule in schedules:
            if schedule.enabled:
                self._add_job(schedule)

    def _add_job(self, schedule) -> None:
        job_id = str(schedule.id)
        trigger = IntervalTrigger(minutes=schedule.interval_minutes)
        self.scheduler.add_job(
            self._run_job,
            trigger,
            id=job_id,
            args=[schedule.id],
            replace_existing=True,
        )
        self._update_next_run(schedule.id)

    def _remove_job(self, schedule_id: int) -> None:
        job_id = str(schedule_id)
        try:
            self.scheduler.remove_job(job_id)
        except Exception:
            pass

    def _update_next_run(self, schedule_id: int) -> None:
        job = self.scheduler.get_job(str(schedule_id))
        if not job:
            return
        next_run = job.next_run_time
        with self.app.app_context():
            update_schedule(schedule_id, next_run_at=next_run)

    def _run_job(self, schedule_id: int) -> None:
        with self.app.app_context():
            schedule = get_schedule(schedule_id)
            if not schedule or not schedule.enabled:
                return
            profile = get_profile(schedule.profile_id)
            if not profile:
                return
            update_schedule(
                schedule.id,
                last_run_at=datetime.now(timezone.utc),
            )
            self._update_next_run(schedule.id)
            options = profile.to_dict()

        # run_profile_func is expected to handle its own app context and concurrency.
        self.run_profile_func(schedule.profile_id, options)

    def add_schedule(self, schedule) -> None:
        self._remove_job(schedule.id)
        if schedule.enabled:
            self._add_job(schedule)

    def remove_schedule(self, schedule_id: int) -> None:
        self._remove_job(schedule_id)
        with self.app.app_context():
            update_schedule(schedule_id, next_run_at=None)

    def update_schedule(self, schedule) -> None:
        if schedule.enabled:
            self.add_schedule(schedule)
        else:
            self.pause_schedule(schedule.id)

    def pause_schedule(self, schedule_id: int) -> None:
        self._remove_job(schedule_id)
        with self.app.app_context():
            update_schedule(schedule_id, next_run_at=None)

    def resume_schedule(self, schedule) -> None:
        self._add_job(schedule)
