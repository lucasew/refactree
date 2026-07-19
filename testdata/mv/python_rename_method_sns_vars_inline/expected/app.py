from types import SimpleNamespace
import types
import copy


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_inline_sub():
    return vars(SimpleNamespace(k=A()))["k"].execute() + vars(SimpleNamespace(k=B()))["k"].run()


def use_inline_get():
    return vars(SimpleNamespace(k=A())).get("k").execute() + vars(SimpleNamespace(k=B())).get("k").run()


def use_inline_types():
    return vars(types.SimpleNamespace(k=A()))["k"].execute() + vars(types.SimpleNamespace(k=B()))["k"].run()


def use_inline_copy():
    return copy.copy(vars(SimpleNamespace(k=A()))["k"]).execute() + copy.copy(vars(SimpleNamespace(k=B()))["k"]).run()


def use_inline_assign():
    xa = vars(SimpleNamespace(k=A()))["k"]
    xb = vars(SimpleNamespace(k=B()))["k"]
    return xa.execute() + xb.run()


def use_inline_dunder():
    return SimpleNamespace(k=A()).__dict__["k"].execute() + SimpleNamespace(k=B()).__dict__["k"].run()


def use_preserves_b():
    return vars(SimpleNamespace(k=B()))["k"].run()
