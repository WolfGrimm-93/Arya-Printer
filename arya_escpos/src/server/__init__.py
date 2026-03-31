"""
Server module for Arya ESCPOS
"""
from .api_server import create_app
from .routes import router

__all__ = [
    "create_app",
    "router",
]
