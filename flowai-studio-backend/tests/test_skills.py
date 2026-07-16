import pytest

from aiflow_runtime.ai.skills import calculate


def test_calculator_uses_ast_allowlist():
    assert calculate("2 + 3 * 4") == 14


def test_calculator_rejects_calls():
    with pytest.raises(ValueError):
        calculate("__import__('os').system('whoami')")
