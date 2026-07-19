from types import SimpleNamespace
import types


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_next_iter_values():
    return (
        next(iter(vars(SimpleNamespace(k=A())).values())).execute()
        + next(iter(vars(SimpleNamespace(k=B())).values())).run()
    )


def use_next_values():
    return (
        next(vars(SimpleNamespace(k=A())).values()).execute()
        + next(vars(SimpleNamespace(k=B())).values()).run()
    )


def use_next_iter_types():
    return (
        next(iter(vars(types.SimpleNamespace(k=A())).values())).execute()
        + next(iter(vars(types.SimpleNamespace(k=B())).values())).run()
    )


def use_next_iter_dunder():
    return (
        next(iter(SimpleNamespace(k=A()).__dict__.values())).execute()
        + next(iter(SimpleNamespace(k=B()).__dict__.values())).run()
    )


def use_next_assign():
    xa = next(iter(vars(SimpleNamespace(k=A())).values()))
    xb = next(iter(vars(SimpleNamespace(k=B())).values()))
    return xa.execute() + xb.run()


def use_values_for():
    s = 0
    for x in vars(SimpleNamespace(k=A())).values():
        s += x.execute()
    for y in vars(SimpleNamespace(k=B())).values():
        s += y.run()
    return s


def use_values_for_types():
    s = 0
    for x in vars(types.SimpleNamespace(k=A())).values():
        s += x.execute()
    for y in vars(types.SimpleNamespace(k=B())).values():
        s += y.run()
    return s


def use_values_for_dunder():
    s = 0
    for x in SimpleNamespace(k=A()).__dict__.values():
        s += x.execute()
    for y in SimpleNamespace(k=B()).__dict__.values():
        s += y.run()
    return s


def use_list_values_for():
    s = 0
    for x in list(vars(SimpleNamespace(k=A())).values()):
        s += x.execute()
    for y in list(vars(SimpleNamespace(k=B())).values()):
        s += y.run()
    return s


def use_preserves_b():
    return next(iter(vars(SimpleNamespace(k=B())).values())).run()


def use_list_values_index():
    return (
        list(vars(SimpleNamespace(k=A())).values())[0].execute()
        + list(vars(SimpleNamespace(k=B())).values())[0].run()
    )


def use_tuple_values_index():
    return (
        tuple(vars(SimpleNamespace(k=A())).values())[0].execute()
        + tuple(vars(SimpleNamespace(k=B())).values())[0].run()
    )


def use_list_values_index_assign():
    xa = list(vars(SimpleNamespace(k=A())).values())[0]
    xb = list(vars(SimpleNamespace(k=B())).values())[0]
    return xa.execute() + xb.run()
