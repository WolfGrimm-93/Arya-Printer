"""
Logging configuration using Loguru with daily rotation and auto-cleanup
"""
import sys
from pathlib import Path
from datetime import datetime, timedelta
from loguru import logger
from typing import Optional
import shutil


class Logger:
    """Centralized logging configuration with daily rotation and auto-cleanup"""
    
    def __init__(
        self,
        level: str = "INFO",
        log_dir: Optional[str] = None,
        retention_days: int = 15,
        format_string: Optional[str] = None,
    ):
        """
        Initialize logger with daily rotation and automatic cleanup
        
        Args:
            level: Log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
            log_dir: Directory for log files (creates daily subfolders)
            retention_days: Number of days to keep logs (default: 15)
            format_string: Custom format string
        """
        self.level = level
        self.log_dir = Path(log_dir) if log_dir else Path("logs")
        self.retention_days = retention_days
        
        # Default format
        if format_string is None:
            format_string = (
                "<green>{time:YYYY-MM-DD HH:mm:ss}</green> | "
                "<level>{level: <8}</level> | "
                "<cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> - "
                "<level>{message}</level>"
            )
        
        self.format_string = format_string
        
        # Remove default handler
        logger.remove()
        
        # Add console handler
        logger.add(
            sys.stderr,
            format=format_string,
            level=level,
            colorize=True,
        )
        
        # Setup daily log file
        self._setup_daily_log()
        
        # Clean old logs
        self._cleanup_old_logs()
    
    def _setup_daily_log(self):
        """Setup daily log file in dated subfolder"""
        # Create logs directory structure: logs/YYYY-MM-DD/arya_escpos.log
        today = datetime.now().strftime("%Y-%m-%d")
        daily_log_dir = self.log_dir / today
        daily_log_dir.mkdir(parents=True, exist_ok=True)
        
        log_file = daily_log_dir / "arya_escpos.log"
        
        # Add file handler with rotation at midnight
        logger.add(
            str(log_file),
            format=self.format_string,
            level=self.level,
            rotation="00:00",  # Rotate at midnight
            retention=f"{self.retention_days} days",
            compression="zip",
            enqueue=True,  # Thread-safe
        )
        
        logger.info(f"Logging initialized - Daily log: {log_file}")
        logger.info(f"Retention policy: {self.retention_days} days")
    
    def _cleanup_old_logs(self):
        """Delete log folders older than retention_days"""
        if not self.log_dir.exists():
            return
        
        cutoff_date = datetime.now() - timedelta(days=self.retention_days)
        deleted_count = 0
        
        # Iterate through date folders
        for folder in self.log_dir.iterdir():
            if not folder.is_dir():
                continue
            
            try:
                # Parse folder name as date (YYYY-MM-DD)
                folder_date = datetime.strptime(folder.name, "%Y-%m-%d")
                
                # Delete if older than retention
                if folder_date < cutoff_date:
                    shutil.rmtree(folder)
                    deleted_count += 1
                    logger.info(f"Deleted old log folder: {folder.name}")
                    
            except ValueError:
                # Skip folders that don't match date format
                continue
        
        if deleted_count > 0:
            logger.info(f"Cleanup completed: {deleted_count} old log folder(s) removed")
    
    @staticmethod
    def get_logger():
        """Get the logger instance"""
        return logger


# Global logger instance
_logger_instance = None


def init_logger(
    level: str = "INFO",
    log_dir: Optional[str] = "logs",
    retention_days: int = 15,
    format_string: Optional[str] = None,
) -> logger:
    """
    Initialize global logger with daily rotation
    
    Args:
        level: Log level
        log_dir: Directory for log files (creates daily subfolders)
        retention_days: Number of days to keep logs (default: 15)
        format_string: Custom format string
        
    Returns:
        Logger instance
    """
    global _logger_instance
    _logger_instance = Logger(level, log_dir, retention_days, format_string)
    return logger


def get_logger() -> logger:
    """
    Get global logger instance
    
    Returns:
        Logger instance
    """
    if _logger_instance is None:
        init_logger()
    return logger
