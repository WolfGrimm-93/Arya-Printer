"""
FastAPI Application Factory
"""
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from contextlib import asynccontextmanager

from server.routes import router
from utils.logger import get_logger


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan events"""
    logger = get_logger()
    
    # Startup
    logger.info("🚀 Application startup")
    
    yield
    
    # Shutdown
    logger.info("👋 Application shutdown")


def create_app() -> FastAPI:
    """
    Create and configure FastAPI application
    
    Returns:
        FastAPI app instance
    """
    app = FastAPI(
        title="Arya ESCPOS API",
        description="API para detección y gestión de dispositivos de impresión ESC/POS",
        version="1.0.0",
        docs_url="/docs",
        redoc_url="/redoc",
        lifespan=lifespan,
    )
    
    # CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=False,
        allow_methods=["*"],
        allow_headers=["*"],
    )
    
    # Include routers
    app.include_router(router, prefix="/api/v1")
    
    @app.get("/", tags=["Health"])
    async def root():
        """API root endpoint"""
        return {
            "service": "Arya ESCPOS",
            "version": "1.0.0",
            "status": "running",
            "docs": "/docs",
        }
    
    @app.get("/health", tags=["Health"])
    async def health_check():
        """Health check endpoint"""
        return {"status": "healthy"}
    
    return app
