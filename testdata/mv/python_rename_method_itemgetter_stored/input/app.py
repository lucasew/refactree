from operator import itemgetter
import operator
from typing import TypedDict


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class Box(TypedDict):
    a: A
    b: B


def use_stored_index(items: list[A], other: list[B]) -> int:
    gia = itemgetter(0)
    gib = itemgetter(0)
    return gia(items).run() + gib(other).run()


def use_operator_stored_index(items: list[A], other: list[B]) -> int:
    gia = operator.itemgetter(0)
    gib = operator.itemgetter(0)
    return gia(items).run() + gib(other).run()


def use_stored_index_assign(items: list[A], other: list[B]) -> int:
    gia = itemgetter(0)
    gib = itemgetter(0)
    a = gia(items)
    b = gib(other)
    return a.run() + b.run()


def use_stored_key(box: Box) -> int:
    gka = itemgetter("a")
    gkb = itemgetter("b")
    return gka(box).run() + gkb(box).run()


def use_operator_stored_key(box: Box) -> int:
    gka = operator.itemgetter("a")
    gkb = operator.itemgetter("b")
    return gka(box).run() + gkb(box).run()


def use_preserves_b(other: list[B]) -> int:
    gib = itemgetter(0)
    return gib(other).run()
