# Contributing to Arya ESCPOS

Gracias por tu interés en contribuir a Arya ESCPOS! 🎉

## 🚀 Cómo Contribuir

### 1. Fork y Clone

```bash
# Fork el repositorio en GitHub
# Luego clona tu fork
git clone https://github.com/TU_USUARIO/arya_escpos.git
cd arya_escpos
```

### 2. Configurar Entorno de Desarrollo

```bash
# Crear entorno virtual
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Instalar dependencias
pip install -r requirements.txt

# Instalar dependencias de desarrollo
pip install pytest pytest-cov pytest-mock black flake8
```

### 3. Crear una Rama

```bash
git checkout -b feature/nombre-de-tu-feature
# o
git checkout -b fix/nombre-del-fix
```

### 4. Hacer Cambios

- Escribe código limpio y documentado
- Sigue las convenciones de estilo (PEP 8)
- Agrega tests para nuevas funcionalidades
- Actualiza la documentación si es necesario

### 5. Ejecutar Tests

```bash
# Ejecutar todos los tests
pytest tests/ -v

# Con cobertura
pytest --cov=src tests/

# Tests específicos
pytest tests/test_adapters.py -v
```

Ver [docs/TESTING.md](docs/TESTING.md) para más detalles.

### 6. Formatear Código

```bash
# Formatear con black
black src/ tests/

# Verificar con flake8
flake8 src/ tests/
```

### 7. Commit y Push

```bash
git add .
git commit -m "feat: descripción clara del cambio"
git push origin feature/nombre-de-tu-feature
```

### 8. Crear Pull Request

1. Ve a GitHub y abre un Pull Request
2. Describe claramente los cambios realizados
3. Referencia cualquier issue relacionado
4. Espera la revisión

## 📝 Convenciones de Commit

Usamos commits semánticos:

- `feat:` - Nueva funcionalidad
- `fix:` - Corrección de bug
- `docs:` - Cambios en documentación
- `style:` - Formateo, punto y coma faltante, etc.
- `refactor:` - Refactorización de código
- `test:` - Agregar o modificar tests
- `chore:` - Mantenimiento, dependencias, etc.

Ejemplos:
```
feat: agregar soporte para impresoras Bluetooth
fix: corregir error en detección USB en Windows
docs: actualizar guía de instalación
test: agregar tests para NetworkAdapter
```

## 🐛 Reportar Bugs

Para reportar un bug, abre un issue con:

1. **Descripción clara** del problema
2. **Pasos para reproducir** el error
3. **Comportamiento esperado** vs. comportamiento actual
4. **Información del sistema** (OS, versión de Python, etc.)
5. **Logs relevantes** si están disponibles

## 💡 Proponer Features

Para proponer una nueva funcionalidad:

1. Abre un issue describiendo la feature
2. Explica el caso de uso
3. Discute la implementación antes de programar
4. Espera feedback antes de hacer un PR grande

## 📋 Checklist antes de PR

- [ ] El código sigue las convenciones de estilo (PEP 8)
- [ ] Los tests pasan (`pytest tests/`)
- [ ] Agregaste tests para nuevas funcionalidades
- [ ] La documentación está actualizada
- [ ] El código está formateado con `black`
- [ ] No hay warnings de `flake8`
- [ ] Los commits son claros y descriptivos

## ❓ Preguntas

Si tienes preguntas, abre un issue o inicia una discusión en GitHub.

## 📜 Licencia

Al contribuir, aceptas que tus contribuciones se licencien bajo la licencia MIT del proyecto.

¡Gracias por contribuir! 🙏
