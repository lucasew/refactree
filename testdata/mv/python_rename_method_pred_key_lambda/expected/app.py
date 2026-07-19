import heapq
import itertools
from itertools import takewhile, dropwhile, filterfalse
from heapq import nlargest, nsmallest


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_takewhile(items: list[A]):
    takewhile(lambda x: x.execute(), items)


def use_takewhile_mod(items: list[A]):
    itertools.takewhile(lambda x: x.execute(), items)


def use_takewhile_b(items: list[B]):
    takewhile(lambda y: y.run(), items)


def use_dropwhile(items: list[A]):
    dropwhile(lambda x: x.execute(), items)


def use_dropwhile_mod(items: list[A]):
    itertools.dropwhile(lambda x: x.execute(), items)


def use_dropwhile_b(items: list[B]):
    dropwhile(lambda y: y.run(), items)


def use_filterfalse(items: list[A]):
    filterfalse(lambda x: x.execute(), items)


def use_filterfalse_mod(items: list[A]):
    itertools.filterfalse(lambda x: x.execute(), items)


def use_filterfalse_b(items: list[B]):
    filterfalse(lambda y: y.run(), items)


def use_nlargest(items: list[A]):
    nlargest(1, items, key=lambda x: x.execute())


def use_nlargest_mod(items: list[A]):
    heapq.nlargest(1, items, key=lambda x: x.execute())


def use_nlargest_b(items: list[B]):
    nlargest(1, items, key=lambda y: y.run())


def use_nsmallest(items: list[A]):
    nsmallest(1, items, key=lambda x: x.execute())


def use_nsmallest_mod(items: list[A]):
    heapq.nsmallest(1, items, key=lambda x: x.execute())


def use_nsmallest_b(items: list[B]):
    nsmallest(1, items, key=lambda y: y.run())


def use_takewhile_assigned():
    xs = [A()]
    takewhile(lambda x: x.execute(), xs)
    ys = [B()]
    takewhile(lambda y: y.run(), ys)


def use_nlargest_literal():
    nlargest(1, [A()], key=lambda x: x.execute())
    heapq.nsmallest(1, [B()], key=lambda y: y.run())
