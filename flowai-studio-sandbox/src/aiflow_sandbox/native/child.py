from __future__ import annotations

import ast
import sys


class PolicyError(ValueError):
    pass


class PolicyValidator(ast.NodeVisitor):
    denied_nodes = (
        ast.AsyncFunctionDef,
        ast.Await,
        ast.ClassDef,
        ast.Global,
        ast.Import,
        ast.ImportFrom,
        ast.Nonlocal,
    )
    denied_calls = {"breakpoint", "compile", "eval", "exec", "input", "open", "__import__"}

    def generic_visit(self, node: ast.AST) -> None:
        if isinstance(node, self.denied_nodes):
            raise PolicyError(f"{type(node).__name__} is not available in the sandbox")
        super().generic_visit(node)

    def visit_Attribute(self, node: ast.Attribute) -> None:
        if node.attr.startswith("_"):
            raise PolicyError("private attributes are not available in the sandbox")
        self.generic_visit(node)

    def visit_Name(self, node: ast.Name) -> None:
        if node.id.startswith("_"):
            raise PolicyError("private names are not available in the sandbox")
        self.generic_visit(node)

    def visit_Call(self, node: ast.Call) -> None:
        if isinstance(node.func, ast.Name) and node.func.id in self.denied_calls:
            raise PolicyError(f"{node.func.id} is not available in the sandbox")
        self.generic_visit(node)


SAFE_BUILTINS = {
    "Exception": Exception,
    "RuntimeError": RuntimeError,
    "TypeError": TypeError,
    "ValueError": ValueError,
    "abs": abs,
    "all": all,
    "any": any,
    "bool": bool,
    "dict": dict,
    "enumerate": enumerate,
    "filter": filter,
    "float": float,
    "int": int,
    "len": len,
    "list": list,
    "map": map,
    "max": max,
    "min": min,
    "print": print,
    "range": range,
    "reversed": reversed,
    "round": round,
    "set": set,
    "sorted": sorted,
    "str": str,
    "sum": sum,
    "tuple": tuple,
    "zip": zip,
}


def main() -> int:
    source = sys.stdin.read()
    try:
        tree = ast.parse(source, mode="exec")
        PolicyValidator().visit(tree)
        code = compile(tree, "<sandbox>", "exec")
        exec(code, {"__builtins__": SAFE_BUILTINS}, {})
    except BaseException as exc:
        print(f"{type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
