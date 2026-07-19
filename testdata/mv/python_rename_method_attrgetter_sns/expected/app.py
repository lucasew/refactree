from operator import attrgetter
import operator
from types import SimpleNamespace
import types


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_attrgetter_sns():
    return (
        attrgetter("k")(SimpleNamespace(k=A())).execute()
        + attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_operator_attrgetter_sns():
    return (
        operator.attrgetter("k")(SimpleNamespace(k=A())).execute()
        + operator.attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_types_sns():
    return (
        attrgetter("k")(types.SimpleNamespace(k=A())).execute()
        + attrgetter("k")(types.SimpleNamespace(k=B())).run()
    )


def use_assign():
    xa = attrgetter("k")(SimpleNamespace(k=A()))
    xb = attrgetter("k")(SimpleNamespace(k=B()))
    return xa.execute() + xb.run()


def use_stored():
    ga = attrgetter("k")
    return (
        ga(SimpleNamespace(k=A())).execute()
        + ga(SimpleNamespace(k=B())).run()
    )


def use_local_sns():
    da = SimpleNamespace(k=A())
    db = SimpleNamespace(k=B())
    return attrgetter("k")(da).execute() + attrgetter("k")(db).run()


def use_preserves_b():
    return attrgetter("k")(SimpleNamespace(k=B())).run()
