from operator import attrgetter
from types import SimpleNamespace
import operator
import types


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    def __init__(self) -> None:
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self) -> None:
        self.b = B()

    def get(self) -> B:
        return self.b


# Class regression — already solid.
def use_class_attrgetter_sns() -> int:
    return (
        attrgetter("k")(SimpleNamespace(k=A())).execute()
        + attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_class_operator_attrgetter_sns() -> int:
    return (
        operator.attrgetter("k")(SimpleNamespace(k=A())).execute()
        + operator.attrgetter("k")(SimpleNamespace(k=B())).run()
    )


def use_class_types_attrgetter_sns() -> int:
    return (
        attrgetter("k")(types.SimpleNamespace(k=A())).execute()
        + attrgetter("k")(types.SimpleNamespace(k=B())).run()
    )


def use_class_getattr_sns() -> int:
    return (
        getattr(SimpleNamespace(k=A()), "k").execute()
        + getattr(SimpleNamespace(k=B()), "k").run()
    )


def use_class_attrgetter_assign() -> int:
    xa = attrgetter("k")(SimpleNamespace(k=A()))
    xb = attrgetter("k")(SimpleNamespace(k=B()))
    return xa.execute() + xb.run()


def use_class_getattr_assign() -> int:
    xa = getattr(SimpleNamespace(k=A()), "k")
    xb = getattr(SimpleNamespace(k=B()), "k")
    return xa.execute() + xb.run()


def use_class_stored_attrgetter() -> int:
    ga = attrgetter("k")
    return (
        ga(SimpleNamespace(k=A())).execute()
        + ga(SimpleNamespace(k=B())).run()
    )


# Method-return under foreign same-leaf.
def use_mr_attrgetter_sns(ba: BoxA, bb: BoxB) -> int:
    return (
        attrgetter("k")(SimpleNamespace(k=ba.get())).execute()
        + attrgetter("k")(SimpleNamespace(k=bb.get())).run()
    )


def use_mr_operator_attrgetter_sns(ba: BoxA, bb: BoxB) -> int:
    return (
        operator.attrgetter("k")(SimpleNamespace(k=ba.get())).execute()
        + operator.attrgetter("k")(SimpleNamespace(k=bb.get())).run()
    )


def use_mr_types_attrgetter_sns(ba: BoxA, bb: BoxB) -> int:
    return (
        attrgetter("k")(types.SimpleNamespace(k=ba.get())).execute()
        + attrgetter("k")(types.SimpleNamespace(k=bb.get())).run()
    )


def use_mr_getattr_sns(ba: BoxA, bb: BoxB) -> int:
    return (
        getattr(SimpleNamespace(k=ba.get()), "k").execute()
        + getattr(SimpleNamespace(k=bb.get()), "k").run()
    )


def use_mr_attrgetter_assign(ba: BoxA, bb: BoxB) -> int:
    xa = attrgetter("k")(SimpleNamespace(k=ba.get()))
    xb = attrgetter("k")(SimpleNamespace(k=bb.get()))
    return xa.execute() + xb.run()


def use_mr_getattr_assign(ba: BoxA, bb: BoxB) -> int:
    xa = getattr(SimpleNamespace(k=ba.get()), "k")
    xb = getattr(SimpleNamespace(k=bb.get()), "k")
    return xa.execute() + xb.run()


def use_mr_stored_attrgetter(ba: BoxA, bb: BoxB) -> int:
    ga = attrgetter("k")
    return (
        ga(SimpleNamespace(k=ba.get())).execute()
        + ga(SimpleNamespace(k=bb.get())).run()
    )


# Inline SNS attribute already worked for method-return.
def use_mr_inline_sns_attr(ba: BoxA, bb: BoxB) -> int:
    return SimpleNamespace(k=ba.get()).k.execute() + SimpleNamespace(k=bb.get()).k.run()


def use_preserves_b(bb: BoxB) -> int:
    return (
        attrgetter("k")(SimpleNamespace(k=bb.get())).run()
        + getattr(SimpleNamespace(k=bb.get()), "k").run()
        + B().run()
    )
