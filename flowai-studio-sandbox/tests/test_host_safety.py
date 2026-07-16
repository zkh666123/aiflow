import ast
from pathlib import Path


def violations_in(files: list[Path], allow_native_boundary: bool) -> list[str]:
    violations: list[str] = []
    for path in files:
        tree = ast.parse(path.read_text(encoding="utf8"), filename=str(path))
        is_native_boundary = allow_native_boundary and "native" in path.parts
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                modules = [alias.name for alias in node.names]
                if isinstance(node, ast.ImportFrom) and node.module:
                    modules.append(node.module)
                if not is_native_boundary and any(
                    name == "subprocess" or name.startswith("subprocess.") for name in modules
                ):
                    violations.append(f"{path}: imports subprocess")
            if not isinstance(node, ast.Call):
                continue
            if isinstance(node.func, ast.Name) and node.func.id == "eval":
                violations.append(f"{path}: calls eval")
            if isinstance(node.func, ast.Name) and node.func.id == "exec" and not is_native_boundary:
                violations.append(f"{path}: calls exec")
            if (
                isinstance(node.func, ast.Attribute)
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "os"
                and node.func.attr == "system"
            ):
                violations.append(f"{path}: calls os.system")
            if any(
                keyword.arg == "shell"
                and isinstance(keyword.value, ast.Constant)
                and keyword.value.value is True
                for keyword in node.keywords
            ):
                violations.append(f"{path}: uses shell=True")
    return violations


def test_web_backend_has_no_native_code_execution_path() -> None:
    backend_root = Path(__file__).parents[2] / "flowai-studio-backend" / "src"
    assert violations_in(list(backend_root.rglob("*.py")), allow_native_boundary=False) == []


def test_native_execution_is_confined_to_the_sandbox_boundary() -> None:
    sandbox_root = Path(__file__).parents[1] / "src" / "aiflow_sandbox"
    assert violations_in(list(sandbox_root.rglob("*.py")), allow_native_boundary=True) == []
