from types import SimpleNamespace
import types
import copy


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_inline_sub():
    return vars(SimpleNamespace(k=A()))["k"].run() + vars(SimpleNamespace(k=B()))["k"].run()


def use_inline_get():
    return vars(SimpleNamespace(k=A())).get("k").run() + vars(SimpleNamespace(k=B())).get("k").run()


def use_inline_types():
    return vars(types.SimpleNamespace(k=A()))["k"].run() + vars(types.SimpleNamespace(k=B()))["k"].run()


def use_inline_copy():
    return copy.copy(vars(SimpleNamespace(k=A()))["k"]).run() + copy.copy(vars(SimpleNamespace(k=B()))["k"]).run()


def use_inline_assign():
    xa = vars(SimpleNamespace(k=A()))["k"]
    xb = vars(SimpleNamespace(k=B()))["k"]
    return xa.run() + xb.run()


def use_inline_dunder():
    return SimpleNamespace(k=A()).__dict__["k"].run() + SimpleNamespace(k=B()).__dict__["k"].run()


def use_preserves_b():
    return vars(SimpleNamespace(k=B()))["k"].run()
