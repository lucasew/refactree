from types import SimpleNamespace
import types


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_items_next():
    return (
        next(iter(vars(SimpleNamespace(k=A())).items()))[1].run()
        + next(iter(vars(SimpleNamespace(k=B())).items()))[1].run()
    )


def use_items_for():
    n = 0
    for _, va in vars(SimpleNamespace(k=A())).items():
        n += va.run()
    for _, vb in vars(SimpleNamespace(k=B())).items():
        n += vb.run()
    return n


def use_items_list():
    return (
        list(vars(SimpleNamespace(k=A())).items())[0][1].run()
        + list(vars(SimpleNamespace(k=B())).items())[0][1].run()
    )


def use_items_types():
    return (
        next(iter(vars(types.SimpleNamespace(k=A())).items()))[1].run()
        + next(iter(vars(types.SimpleNamespace(k=B())).items()))[1].run()
    )


def use_items_dunder():
    return (
        next(iter(SimpleNamespace(k=A()).__dict__.items()))[1].run()
        + next(iter(SimpleNamespace(k=B()).__dict__.items()))[1].run()
    )


def use_preserves_b():
    return next(iter(vars(SimpleNamespace(k=B())).items()))[1].run()
