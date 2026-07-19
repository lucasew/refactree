from operator import attrgetter
import operator
from dataclasses import dataclass


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_stored_chain(box: Box) -> int:
    ga = attrgetter("a")
    gb = attrgetter("b")
    return ga(box).execute() + gb(box).run()


def use_operator_stored_chain(box: Box) -> int:
    ga = operator.attrgetter("a")
    gb = operator.attrgetter("b")
    return ga(box).execute() + gb(box).run()


def use_stored_assign(box: Box) -> int:
    ga = attrgetter("a")
    gb = attrgetter("b")
    xa = ga(box)
    xb = gb(box)
    return xa.execute() + xb.run()


def use_stored_walrus(box: Box) -> int:
    ga = attrgetter("a")
    if (xa := ga(box)):
        return xa.execute()
    return 0


def use_preserves_b(box: Box) -> int:
    gb = attrgetter("b")
    return gb(box).run()
