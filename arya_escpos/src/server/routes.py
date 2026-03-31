"""API Routes for Arya ESCPOS - aggregates all sub-routers."""
from fastapi import APIRouter

from server.device_routes import router as device_router
from server.print_routes import router as print_router
from server.config_routes import router as config_router

router = APIRouter()
router.include_router(device_router)
router.include_router(print_router)
router.include_router(config_router)
