"""Configuration routes"""
from fastapi import APIRouter
from utils.config import get_config

router = APIRouter()


@router.get("/config", tags=["Configuration"])
async def get_configuration():
    """Get current configuration"""
    config = get_config()
    return config.to_dict()
