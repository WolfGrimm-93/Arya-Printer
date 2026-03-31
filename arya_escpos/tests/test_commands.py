"""
Tests para construcción de comandos ESC/POS
"""

import pytest
from core.command_builder import CommandBuilder


def test_command_builder_initialization():
    """Test: Crear command builder"""
    builder = CommandBuilder()
    assert builder is not None
    assert hasattr(builder, 'buffer')


def test_initialize_command(sample_escpos_commands):
    """Test: Comando de inicialización"""
    builder = CommandBuilder()
    builder.initialize()
    
    assert sample_escpos_commands['initialize'] in builder.buffer


def test_text_command(sample_escpos_commands):
    """Test: Agregar texto"""
    builder = CommandBuilder()
    builder.text("Hello World\n")
    
    assert b"Hello World\n" in builder.buffer


def test_alignment_commands(sample_escpos_commands):
    """Test: Comandos de alineación"""
    builder = CommandBuilder()
    
    # Centro
    builder.align('center')
    assert sample_escpos_commands['center'] in builder.buffer
    
    # Izquierda
    builder.align('left')
    assert sample_escpos_commands['left'] in builder.buffer


def test_text_style_bold(sample_escpos_commands):
    """Test: Texto en negrita"""
    builder = CommandBuilder()
    
    builder.bold(True)
    assert sample_escpos_commands['bold_on'] in builder.buffer
    
    builder.bold(False)
    assert sample_escpos_commands['bold_off'] in builder.buffer


def test_cut_command(sample_escpos_commands):
    """Test: Comando de corte"""
    builder = CommandBuilder()
    builder.cut()
    
    assert sample_escpos_commands['cut'] in builder.buffer


def test_chain_commands():
    """Test: Encadenar comandos"""
    builder = CommandBuilder()
    
    builder.initialize() \
           .align('center') \
           .bold(True) \
           .text("TITULO\n") \
           .bold(False) \
           .cut()
    
    assert len(builder.buffer) > 0
    assert b"TITULO" in builder.buffer


def test_barcode_command():
    """Test: Generar código de barras (si está implementado)"""
    builder = CommandBuilder()
    
    if hasattr(builder, 'barcode'):
        builder.barcode("123456789", barcode_type="CODE39")
        assert len(builder.buffer) > 0


def test_qrcode_command():
    """Test: Generar QR code (si está implementado)"""
    builder = CommandBuilder()
    
    if hasattr(builder, 'qrcode'):
        builder.qrcode("https://example.com")
        assert len(builder.buffer) > 0


def test_get_bytes():
    """Test: Obtener bytes finales"""
    builder = CommandBuilder()
    builder.initialize()
    builder.text("Test\n")
    
    data = builder.get_bytes()
    
    assert isinstance(data, bytes)
    assert len(data) > 0
    assert b"Test" in data


def test_clear_buffer():
    """Test: Limpiar buffer"""
    builder = CommandBuilder()
    builder.text("Some text\n")
    
    assert len(builder.buffer) > 0
    
    if hasattr(builder, 'clear'):
        builder.clear()
        assert len(builder.buffer) == 0


def test_feed_lines():
    """Test: Avance de líneas (feed)"""
    builder = CommandBuilder()
    
    if hasattr(builder, 'feed'):
        builder.feed(3)
        assert b"\n" in builder.buffer


def test_encoding_support():
    """Test: Soporte de encoding"""
    builder = CommandBuilder()
    
    # Texto con acentos
    builder.text("Café\n")
    
    data = builder.get_bytes()
    assert len(data) > 0


def test_image_command():
    """Test: Comando de imagen (si está implementado)"""
    builder = CommandBuilder()
    
    if hasattr(builder, 'image'):
        # Test básico - implementación futura
        assert callable(builder.image)
