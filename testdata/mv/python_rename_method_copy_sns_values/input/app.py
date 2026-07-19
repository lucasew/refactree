import copy
from types import SimpleNamespace
import types


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_copy_next_values():
    return (
        copy.copy(next(iter(vars(SimpleNamespace(k=A())).values()))).run()
        + copy.copy(next(iter(vars(SimpleNamespace(k=B())).values()))).run()
    )


def use_copy_next_direct():
    return (
        copy.copy(next(vars(SimpleNamespace(k=A())).values())).run()
        + copy.copy(next(vars(SimpleNamespace(k=B())).values())).run()
    )


def use_copy_next_types():
    return (
        copy.copy(next(iter(vars(types.SimpleNamespace(k=A())).values()))).run()
        + copy.copy(next(iter(vars(types.SimpleNamespace(k=B())).values()))).run()
    )


def use_copy_next_dunder():
    return (
        copy.copy(next(iter(SimpleNamespace(k=A()).__dict__.values()))).run()
        + copy.copy(next(iter(SimpleNamespace(k=B()).__dict__.values()))).run()
    )


def use_copy_assign():
    xa = copy.copy(next(iter(vars(SimpleNamespace(k=A())).values())))
    xb = copy.copy(next(iter(vars(SimpleNamespace(k=B())).values())))
    return xa.run() + xb.run()


def use_deepcopy():
    return (
        copy.deepcopy(next(iter(vars(SimpleNamespace(k=A())).values()))).run()
        + copy.deepcopy(next(iter(vars(SimpleNamespace(k=B())).values()))).run()
    )


def use_copy_list_index():
    return (
        copy.copy(list(vars(SimpleNamespace(k=A())).values())[0]).run()
        + copy.copy(list(vars(SimpleNamespace(k=B())).values())[0]).run()
    )



def use_copy_getattr():
    return (
        copy.copy(getattr(SimpleNamespace(k=A()), "k")).run()
        + copy.copy(getattr(SimpleNamespace(k=B()), "k")).run()
    )


def use_copy_getattr_types():
    return (
        copy.copy(getattr(types.SimpleNamespace(k=A()), "k")).run()
        + copy.copy(getattr(types.SimpleNamespace(k=B()), "k")).run()
    )

def use_preserves_b():
    return copy.copy(next(iter(vars(SimpleNamespace(k=B())).values()))).run()
