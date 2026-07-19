from operator import attrgetter
import operator
from types import SimpleNamespace
import types


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_attrgetter_sns():
    return (
        attrgetter("k")(SimpleNamespace(k=A())).run()
        + attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_operator_attrgetter_sns():
    return (
        operator.attrgetter("k")(SimpleNamespace(k=A())).run()
        + operator.attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_types_sns():
    return (
        attrgetter("k")(types.SimpleNamespace(k=A())).run()
        + attrgetter("k")(types.SimpleNamespace(k=B())).run()
    )


def use_assign():
    xa = attrgetter("k")(SimpleNamespace(k=A()))
    xb = attrgetter("k")(SimpleNamespace(k=B()))
    return xa.run() + xb.run()


def use_stored():
    ga = attrgetter("k")
    return (
        ga(SimpleNamespace(k=A())).run()
        + ga(SimpleNamespace(k=B())).run()
    )


def use_local_sns():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return attrgetter("k")(da).run() + attrgetter("k")(db).run()


def use_preserves_b():
    return attrgetter("k")(SimpleNamespace(k=B())).run()
