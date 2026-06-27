"""Database models and helpers for the parser UI."""
import json
from datetime import datetime, timezone
from typing import Any

from flask_sqlalchemy import SQLAlchemy
from sqlalchemy import JSON, Boolean, DateTime, ForeignKey, Integer, String, Text
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


db = SQLAlchemy(model_class=Base)


def now_utc() -> datetime:
    return datetime.now(timezone.utc)


class Profile(db.Model):
    __tablename__ = "profiles"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    name: Mapped[str] = mapped_column(String(120), nullable=False)
    query: Mapped[str] = mapped_column(String(255), nullable=False, default="chatgpt plus")
    category_id: Mapped[int] = mapped_column(Integer, nullable=False, default=1355)
    candidates: Mapped[int] = mapped_column(Integer, nullable=False, default=40)
    max_pages: Mapped[int | None] = mapped_column(Integer, nullable=True)
    deep: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc, onupdate=now_utc)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "name": self.name,
            "query": self.query,
            "category_id": self.category_id,
            "candidates": self.candidates,
            "max_pages": self.max_pages,
            "deep": self.deep,
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None,
        }


class SavedResult(db.Model):
    __tablename__ = "saved_results"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    profile_id: Mapped[int] = mapped_column(Integer, ForeignKey("profiles.id"), nullable=False)
    run_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc)
    cheapest_json: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    summary_json: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    all_results_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc)

    def to_dict(self, include_results: bool = False) -> dict:
        data = {
            "id": self.id,
            "profile_id": self.profile_id,
            "run_at": self.run_at.isoformat() if self.run_at else None,
            "cheapest": self.cheapest_json,
            "summary": self.summary_json,
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }
        if include_results:
            data["all_results"] = json.loads(self.all_results_json) if self.all_results_json else []
        return data


class Schedule(db.Model):
    __tablename__ = "schedules"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    profile_id: Mapped[int] = mapped_column(Integer, ForeignKey("profiles.id"), nullable=False)
    interval_minutes: Mapped[int] = mapped_column(Integer, nullable=False, default=60)
    enabled: Mapped[bool] = mapped_column(Boolean, nullable=False, default=True)
    next_run_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    last_run_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=now_utc, onupdate=now_utc)

    def to_dict(self, include_next_run: bool = True) -> dict:
        data = {
            "id": self.id,
            "profile_id": self.profile_id,
            "interval_minutes": self.interval_minutes,
            "enabled": self.enabled,
            "last_run_at": self.last_run_at.isoformat() if self.last_run_at else None,
            "created_at": self.created_at.isoformat() if self.created_at else None,
            "updated_at": self.updated_at.isoformat() if self.updated_at else None,
        }
        if include_next_run:
            data["next_run_at"] = self.next_run_at.isoformat() if self.next_run_at else None
        return data


def create_profile(
    name: str,
    query: str,
    category_id: int,
    candidates: int,
    deep: bool,
    max_pages: int | None = None,
) -> Profile:
    profile = Profile(
        name=name,
        query=query,
        category_id=category_id,
        candidates=candidates,
        deep=deep,
        max_pages=max_pages,
    )
    db.session.add(profile)
    db.session.commit()
    return profile


def update_profile(profile_id: int, **kwargs) -> Profile | None:
    profile = db.session.get(Profile, profile_id)
    if not profile:
        return None
    for key, value in kwargs.items():
        if not hasattr(profile, key):
            continue
        setattr(profile, key, value)
    db.session.commit()
    return profile


def delete_profile(profile_id: int) -> bool:
    profile = db.session.get(Profile, profile_id)
    if not profile:
        return False
    db.session.delete(profile)
    db.session.commit()
    return True


def list_profiles() -> list[Profile]:
    return db.session.execute(
        db.select(Profile).order_by(Profile.updated_at.desc())
    ).scalars().all()


def get_profile(profile_id: int) -> Profile | None:
    return db.session.get(Profile, profile_id)


def save_result(
    profile_id: int,
    cheapest: dict | None,
    summary: dict | None,
    all_results: list[dict],
) -> SavedResult:
    result = SavedResult(
        profile_id=profile_id,
        cheapest_json=cheapest,
        summary_json=summary,
        all_results_json=json.dumps(all_results, ensure_ascii=False),
    )
    db.session.add(result)
    db.session.commit()
    return result


def list_saved_results(profile_id: int | None = None) -> list[SavedResult]:
    stmt = db.select(SavedResult).order_by(SavedResult.run_at.desc())
    if profile_id is not None:
        stmt = stmt.where(SavedResult.profile_id == profile_id)
    return db.session.execute(stmt).scalars().all()


def get_saved_result(result_id: int) -> SavedResult | None:
    return db.session.get(SavedResult, result_id)


def delete_saved_result(result_id: int) -> bool:
    result = db.session.get(SavedResult, result_id)
    if not result:
        return False
    db.session.delete(result)
    db.session.commit()
    return True


def create_schedule(
    profile_id: int,
    interval_minutes: int,
    enabled: bool = True,
) -> Schedule:
    schedule = Schedule(
        profile_id=profile_id,
        interval_minutes=interval_minutes,
        enabled=enabled,
    )
    db.session.add(schedule)
    db.session.commit()
    return schedule


def get_schedule(schedule_id: int) -> Schedule | None:
    return db.session.get(Schedule, schedule_id)


def list_schedules() -> list[Schedule]:
    return db.session.execute(
        db.select(Schedule).order_by(Schedule.updated_at.desc())
    ).scalars().all()


_UNSET = object()


def update_schedule(
    schedule_id: int,
    interval_minutes: int | None | object = _UNSET,
    enabled: bool | None | object = _UNSET,
    next_run_at: datetime | None | object = _UNSET,
    last_run_at: datetime | None | object = _UNSET,
) -> Schedule | None:
    schedule = db.session.get(Schedule, schedule_id)
    if not schedule:
        return None
    if interval_minutes is not _UNSET:
        schedule.interval_minutes = interval_minutes
    if enabled is not _UNSET:
        schedule.enabled = enabled
    if next_run_at is not _UNSET:
        schedule.next_run_at = next_run_at
    if last_run_at is not _UNSET:
        schedule.last_run_at = last_run_at
    db.session.commit()
    return schedule


def delete_schedule(schedule_id: int) -> bool:
    schedule = db.session.get(Schedule, schedule_id)
    if not schedule:
        return False
    db.session.delete(schedule)
    db.session.commit()
    return True


def init_db(app: Any) -> None:
    db.init_app(app)
    with app.app_context():
        db.create_all()
        _drop_obsolete_columns()


def _drop_obsolete_columns() -> None:
    """Drop columns removed from the Profile model (price_only, mode)."""
    for column in ("price_only", "mode"):
        try:
            with db.engine.connect() as conn:
                conn.execute(db.text(f'ALTER TABLE profiles DROP COLUMN {column}'))
                conn.commit()
        except Exception:
            pass
