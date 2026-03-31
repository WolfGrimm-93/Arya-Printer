# 🧪 Guía Completa de Testing - Arya ESCPOS

## 📋 Índice

1. [Preparación del Entorno](#preparación)
2. [Estructura de Tests](#estructura)
3. [Ejecución Paso a Paso](#ejecución)
4. [Tests por Componente](#componentes)
5. [Tests de Integración](#integración)
6. [Troubleshooting](#troubleshooting)

---

## 📦 1. Preparación del Entorno {#preparación}

### Paso 1: Activar entorno virtual

```powershell
cd "ruta\del\proyecto\arya_escpos"

# Activar venv
.\venv\Scripts\activate
```

### Paso 2: Instalar dependencias de testing

```powershell
# Si no has instalado requirements.txt
pip install -r requirements.txt

# Instalar dependencias adicionales para tests completos
pip install pytest-mock pytest-cov
```

### Paso 3: Verificar instalación

```powershell
# Verificar pytest
pytest --version

# Debe mostrar: pytest 7.4.4
```

---

## 📁 2. Estructura de Tests {#estructura}

```
tests/
├── __init__.py                 # Inicialización del paquete
├── conftest.py                 # Fixtures compartidos
├── test_config.py              # ✅ Tests de configuración
├── test_database.py            # ✅ Tests de base de datos
├── test_adapters.py            # ✅ Tests de adaptadores
├── test_api.py                 # ✅ Tests de API REST
├── test_scanners.py            # ✅ Tests de scanners
└── test_commands.py            # ✅ Tests de ESC/POS commands
```

### Fixtures Disponibles (conftest.py)

- `test_settings` - Configuración de prueba
- `db_session` - Base de datos en memoria
- `client` - Cliente FastAPI para tests de API
- `mock_usb_device` - Mock de dispositivo USB
- `mock_bluetooth_device` - Mock de dispositivo Bluetooth
- `mock_network_printer` - Mock de impresora de red
- `sample_escpos_commands` - Comandos ESC/POS de ejemplo

---

## 🚀 3. Ejecución Paso a Paso {#ejecución}

### PASO 1: Test Básico - Configuración

```powershell
# Ejecutar solo tests de configuración
pytest tests/test_config.py -v

# ✅ Esperado: 6 tests passed
```

**Qué verifica:**
- Creación de settings
- Override de configuración
- Validación de datos
- Carga desde YAML

---

### PASO 2: Test de Base de Datos

```powershell
# Tests de modelos y BD
pytest tests/test_database.py -v

# ✅ Esperado: 10+ tests passed
```

**Qué verifica:**
- Creación de dispositivos
- Timestamps automáticos
- Constraints de BD
- Queries y filtros
- Dispositivos Bluetooth, USB, Network

---

### PASO 3: Test de Adaptadores

```powershell
# Tests de adaptadores (con mocks)
pytest tests/test_adapters.py -v

# ✅ Esperado: 15+ tests passed
# ⚠️ Algunos tests se saltarán si no tienes hardware
```

**Qué verifica:**
- Inicialización de adaptadores
- USBAdapter, NetworkAdapter, SerialAdapter, BluetoothAdapter
- Context managers
- Validaciones de parámetros
- Mocks de operaciones

---

### PASO 4: Test de API REST

```powershell
# Tests de endpoints FastAPI
pytest tests/test_api.py -v

# ✅ Esperado: 15+ tests passed
```

**Qué verifica:**
- Endpoints GET/POST/DELETE
- `/api/v1/devices`
- `/api/v1/devices/scan`
- OpenAPI schema
- Documentación Swagger
- Manejo de errores

---

### PASO 5: Test de Scanners

```powershell
# Tests de scanners con caché
pytest tests/test_scanners.py -v

# ⚠️ Algunos tests requieren PyBluez
```

**Qué verifica:**
- BluetoothScanner
- Sistema de caché
- Búsqueda por dirección/nombre
- TTL del caché

---

### PASO 6: Test de Comandos ESC/POS

```powershell
# Tests de command builder
pytest tests/test_commands.py -v

# ✅ Esperado: 10+ tests passed
```

**Qué verifica:**
- Construcción de comandos
- Inicialización, texto, corte
- Alineación, estilos
- Encadenamiento de comandos
- Generación de bytes

---

## 🎯 4. Ejecutar TODOS los Tests {#todos}

### Opción A: Ejecución Simple

```powershell
# Todos los tests
pytest

# Con verbose
pytest -v

# Con resumen detallado
pytest -v --tb=short
```

### Opción B: Con Cobertura de Código

```powershell
# Generar reporte de cobertura
pytest --cov=src --cov-report=html --cov-report=term

# Ver reporte en navegador
start htmlcov/index.html
```

### Opción C: Tests Paralelos (más rápido)

```powershell
# Instalar pytest-xdist
pip install pytest-xdist

# Ejecutar en paralelo
pytest -n auto
```

---

## 📊 5. Tests por Componente {#componentes}

### Solo Tests de USB

```powershell
pytest tests/test_adapters.py::test_usb_adapter_initialization -v
pytest tests/test_adapters.py -k "usb" -v
```

### Solo Tests de Bluetooth

```powershell
pytest tests/test_adapters.py -k "bluetooth" -v
pytest tests/test_scanners.py -v
```

### Solo Tests de API

```powershell
pytest tests/test_api.py -v
```

### Solo Tests de Base de Datos

```powershell
pytest tests/test_database.py -v
```

---

## 🔗 6. Tests de Integración {#integración}

### Test Completo: Scan → BD → API

```powershell
# Este test verifica el flujo completo
pytest tests/test_api.py::test_create_device_via_scan -v -s
```

**Flujo que verifica:**
1. Escaneo de dispositivos (mocked)
2. Registro en base de datos
3. Respuesta de API correcta
4. Persistencia de datos

### Test E2E: Simulación Real

```powershell
# Crear archivo de test E2E
pytest tests/test_integration.py -v
```

---

## 🐛 7. Troubleshooting {#troubleshooting}

### Error: "No module named 'src'"

```powershell
# Solución: Ejecutar desde raíz del proyecto
cd "c:\Users\mferr\Downloads\Pos Printer\arya_escpos"
pytest
```

### Error: "PyBluez not installed"

```powershell
# Solución: Instalar PyBluez o skip tests
pip install pybluez

# O ejecutar sin tests de Bluetooth
pytest -k "not bluetooth"
```

### Error: "Database locked"

```powershell
# Solución: Tests usan SQLite en memoria, no debe pasar
# Si pasa, verificar que db_session fixture funciona
pytest tests/test_database.py::test_create_device -v
```

### Tests muy lentos

```powershell
# Solución: Usar pytest-xdist
pip install pytest-xdist
pytest -n 4  # 4 workers paralelos
```

### Ver output de print()

```powershell
# Usar flag -s
pytest -s -v
```

---

## 📈 8. Métricas de Calidad

### Cobertura Mínima Esperada

```
src/utils/          95%+
src/database/       90%+
src/adapters/       75%+ (hardware dependency)
src/server/         85%+
src/core/           90%+
```

### Generar Reporte de Cobertura

```powershell
pytest --cov=src --cov-report=term-missing
```

---

## ✅ 9. Checklist de Testing Completo

### Tests Unitarios
- [ ] test_config.py - Configuración
- [ ] test_database.py - Modelos y BD
- [ ] test_adapters.py - Todos los adaptadores
- [ ] test_commands.py - ESC/POS commands
- [ ] test_scanners.py - Scanners con caché

### Tests de Integración
- [ ] test_api.py - API REST completa
- [ ] Scan → BD → API flow
- [ ] WebSocket (si implementado)

### Tests con Hardware Real (opcional)
- [ ] USB con impresora real
- [ ] Network con impresora IP
- [ ] Bluetooth con dispositivo real
- [ ] Serial con puerto COM real

---

## 🎓 10. Comandos Útiles Resumen

```powershell
# Ejecutar todos
pytest

# Solo un archivo
pytest tests/test_config.py

# Solo una función
pytest tests/test_config.py::test_settings_creation

# Por keyword
pytest -k "bluetooth"

# Verbose + output
pytest -v -s

# Con cobertura
pytest --cov=src --cov-report=html

# Paralelo
pytest -n auto

# Solo failed del último run
pytest --lf

# Ver tests sin ejecutar
pytest --collect-only

# Ver cuánto tarda cada test
pytest --durations=10
```

---

## 🚀 11. Ejecución Recomendada (Primera Vez)

```powershell
# 1. Activar venv
.\venv\Scripts\activate

# 2. Instalar dependencias
pip install -r requirements.txt
pip install pytest-mock pytest-cov pytest-xdist

# 3. Test rápido
pytest tests/test_config.py -v

# 4. Test completo
pytest -v

# 5. Con cobertura
pytest --cov=src --cov-report=html
start htmlcov/index.html

# 6. Guardar resultados
pytest --html=report.html --self-contained-html
```

---

## 📞 Siguiente Paso

**¿Listo para empezar?**

1. Ejecuta: `pytest tests/test_config.py -v`
2. Si pasa, ejecuta: `pytest -v`
3. Revisa el reporte de cobertura
4. Identifica áreas que necesitan más tests

**¿Algún test falla?**
- Revisa el output con `-v -s`
- Usa `--tb=long` para ver traceback completo
- Ejecuta solo ese test para debuggear

---

## 🎯 Objetivo: 85%+ de Cobertura

```powershell
pytest --cov=src --cov-report=term --cov-fail-under=85
```

Este comando fallará si la cobertura es menor al 85%, forzando calidad de código.

---

**¡Ahora tienes todo listo para testing completo! 🚀**
