"""
Database models for Arya ESCPOS
SQLAlchemy ORM models for device persistence
"""
from datetime import datetime
from typing import Optional
from sqlalchemy import Column, String, Integer, DateTime, Boolean, ForeignKey, Text
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import relationship

Base = declarative_base()


class Device(Base):
    """Main device table - stores all detected devices"""
    
    __tablename__ = "devices"
    
    id = Column(String, primary_key=True)  # usb-SERIAL or usb-VID:PID-INDEX
    type = Column(String, nullable=False)  # usb, network, serial, bluetooth
    vid = Column(String, nullable=True)  # Vendor ID (for USB)
    pid = Column(String, nullable=True)  # Product ID (for USB)
    serial_number = Column(String, nullable=True)  # Device serial number
    manufacturer = Column(String, nullable=True)
    product = Column(String, nullable=True)
    alias = Column(String, nullable=True)  # User-friendly name
    last_port = Column(String, nullable=True)  # Last known port/path
    first_seen = Column(DateTime, default=datetime.utcnow)
    last_seen = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    is_active = Column(Boolean, default=True)
    
    # Relationships
    print_history = relationship("PrintHistory", back_populates="device", cascade="all, delete-orphan")
    
    def __repr__(self):
        return f"<Device(id='{self.id}', type='{self.type}', alias='{self.alias}')>"


class Configuration(Base):
    """Configuration key-value store"""
    
    __tablename__ = "configurations"
    
    key = Column(String, primary_key=True)
    value = Column(Text, nullable=False)
    description = Column(String, nullable=True)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    def __repr__(self):
        return f"<Configuration(key='{self.key}', value='{self.value[:50]}...')>"


class NetworkDevice(Base):
    """Network devices (manually configured or discovered)"""
    
    __tablename__ = "network_devices"
    
    id = Column(String, primary_key=True)  # net-IP:PORT
    ip = Column(String, nullable=False)
    port = Column(Integer, nullable=False, default=9100)
    name = Column(String, nullable=True)
    enabled = Column(Boolean, default=True)
    auto_discovered = Column(Boolean, default=False)
    last_tested = Column(DateTime, nullable=True)
    is_reachable = Column(Boolean, default=False)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    
    def __repr__(self):
        return f"<NetworkDevice(id='{self.id}', ip='{self.ip}', port={self.port})>"


class PrintHistory(Base):
    """Print job history (optional, for auditing)"""
    
    __tablename__ = "print_history"
    
    id = Column(Integer, primary_key=True, autoincrement=True)
    device_id = Column(String, ForeignKey("devices.id"), nullable=False)
    timestamp = Column(DateTime, default=datetime.utcnow)
    status = Column(String, nullable=False)  # success, error, pending
    data_size = Column(Integer, nullable=True)  # bytes
    error_message = Column(Text, nullable=True)
    extra_data = Column(Text, nullable=True)  # JSON string with additional info (renamed from metadata)
    document_type = Column(String, nullable=True)  # receipt, label, etc.
    
    # Relationships
    device = relationship("Device", back_populates="print_history")
    
    def __repr__(self):
        return f"<PrintHistory(id={self.id}, device_id='{self.device_id}', status='{self.status}')>"
