from typing import TypedDict
from operator import itemgetter
import operator


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class Box(TypedDict):
    a: A
    b: B


def use_itemgetter(box: Box) -> int:
    xa = itemgetter("a")(box)
    xb = itemgetter("b")(box)
    return xa.run() + xb.run()


def use_operator_itemgetter(box: Box) -> int:
    xa = operator.itemgetter("a")(box)
    xb = operator.itemgetter("b")(box)
    return xa.run() + xb.run()


def use_chain(box: Box) -> int:
    return itemgetter("a")(box).run() + itemgetter("b")(box).run()


def use_walrus(box: Box) -> int:
    if (xa := itemgetter("a")(box)):
        return xa.run()
    return 0
