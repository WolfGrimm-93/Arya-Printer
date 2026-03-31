"""
Database initialization and session management
"""
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, Session
from sqlalchemy.pool import StaticPool
from contextlib import contextmanager
from typing import Generator
import os

from .models import Base


class Database:
    """Database manager for SQLite"""
    
    def __init__(self, database_url: str = "sqlite:///./arya_escpos.db", echo: bool = False):
        """
        Initialize database connection
        
        Args:
            database_url: SQLAlchemy database URL
            echo: Enable SQL query logging
        """
        self.database_url = database_url
        
        # Configure engine
        connect_args = {"check_same_thread": False} if "sqlite" in database_url else {}
        
        self.engine = create_engine(
            database_url,
            echo=echo,
            connect_args=connect_args,
            poolclass=StaticPool if "sqlite" in database_url else None,
        )
        
        self.SessionLocal = sessionmaker(
            autocommit=False,
            autoflush=False,
            bind=self.engine
        )
    
    def create_tables(self):
        """Create all tables if they don't exist"""
        Base.metadata.create_all(bind=self.engine)
    
    def drop_tables(self):
        """Drop all tables (use with caution!)"""
        Base.metadata.drop_all(bind=self.engine)
    
    @contextmanager
    def get_session(self) -> Generator[Session, None, None]:
        """
        Context manager for database sessions
        
        Usage:
            with db.get_session() as session:
                device = session.query(Device).first()
        """
        session = self.SessionLocal()
        try:
            yield session
            session.commit()
        except Exception:
            session.rollback()
            raise
        finally:
            session.close()
    
    def get_db(self) -> Generator[Session, None, None]:
        """
        Dependency injection for FastAPI
        
        Usage:
            @app.get("/devices")
            def get_devices(db: Session = Depends(database.get_db)):
                return db.query(Device).all()
        """
        session = self.SessionLocal()
        try:
            yield session
        finally:
            session.close()


# Global database instance
_db_instance = None


def init_database(database_url: str = "sqlite:///./arya_escpos.db", echo: bool = False) -> Database:
    """
    Initialize global database instance
    
    Args:
        database_url: SQLAlchemy database URL
        echo: Enable SQL query logging
        
    Returns:
        Database instance
    """
    global _db_instance
    _db_instance = Database(database_url, echo)
    _db_instance.create_tables()
    return _db_instance


def get_database() -> Database:
    """
    Get global database instance
    
    Returns:
        Database instance
        
    Raises:
        RuntimeError: If database not initialized
    """
    if _db_instance is None:
        raise RuntimeError("Database not initialized. Call init_database() first.")
    return _db_instance
