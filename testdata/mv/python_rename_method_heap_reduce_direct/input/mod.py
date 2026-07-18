import heapq
from heapq import heappop, heappushpop, heapreplace
from functools import reduce
import functools


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_heappop_direct(items: list[A]) -> int:
    return heappop(items).run()


def use_heapq_heappop_direct(items: list[A]) -> int:
    return heapq.heappop(items).run()


def use_heappushpop_direct(items: list[A], x: A) -> int:
    return heappushpop(items, x).run()


def use_heapreplace_direct(items: list[A], x: A) -> int:
    return heapq.heapreplace(items, x).run()


def use_reduce_direct(items: list[A]) -> int:
    return reduce(lambda x, y: x, items).run()


def use_functools_reduce_direct(items: list[A]) -> int:
    return functools.reduce(lambda x, y: x, items).run()


def use_reduce_init_direct(items: list[A], da: A) -> int:
    return reduce(lambda x, y: x, items, da).run()


def use_heappop_assign(items: list[A]) -> int:
    # assignment path still works (regression)
    a = heappop(items)
    return a.run()


def use_reduce_assign(items: list[A]) -> int:
    a = reduce(lambda x, y: x, items)
    return a.run()


def use_b_heappop(others: list[B]) -> int:
    return heappop(others).run()


def use_b_reduce(bobs: list[B]) -> int:
    return reduce(lambda x, y: x, bobs).run()
