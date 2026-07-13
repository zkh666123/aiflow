import ast
from pathlib import Path


def test_host_source_has_no_native_code_execution_path() -> None:
    source_root = Path(__file__).parents[1] / "src" / "aiflow_sandbox"
    files = list(source_root.rglob("*.py"))
    assert files, "sandbox production source must exist"

    violations: list[str] = []
    for path in files:
        tree = ast.parse(path.read_text(encoding="utf8"), filename=str(path))
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                modules = [alias.name for alias in node.names]
                if isinstance(node, ast.ImportFrom) and node.module:
                    modules.append(node.module)
                if any(name == "subprocess" or name.startswith("subprocess.") for name in modules):
                    violations.append(f"{path}: imports subprocess")
            if isinstance(node, ast.Call):
                if isinstance(node.func, ast.Name) and node.func.id in {"eval", "exec"}:
                    violations.append(f"{path}: calls {node.func.id}")
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

    assert violations == []
