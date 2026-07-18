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


def use_attrgetter(box: Box) -> int:
    xa = attrgetter("a")(box)
    xb = attrgetter("b")(box)
    return xa.execute() + xb.run()


def use_operator_attrgetter(box: Box) -> int:
    xa = operator.attrgetter("a")(box)
    xb = operator.attrgetter("b")(box)
    return xa.execute() + xb.run()


def use_chain(box: Box) -> int:
    return attrgetter("a")(box).execute() + attrgetter("b")(box).run()


def use_walrus(box: Box) -> int:
    if (xa := attrgetter("a")(box)):
        return xa.execute()
    return 0
