import heapq
from heapq import heappushpop, heapreplace


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_heappushpop(items: list[A], x: A):
    a = heapq.heappushpop(items, x)
    a.run()


def use_heappushpop_imported(items: list[A], x: A):
    a = heappushpop(items, x)
    a.run()


def use_heapreplace(items: list[A], x: A):
    a = heapq.heapreplace(items, x)
    a.run()


def use_heapreplace_imported(items: list[A], x: A):
    a = heapreplace(items, x)
    a.run()


def use_heappushpop_b(items: list[B], x: B):
    b = heapq.heappushpop(items, x)
    b.run()


def use_heapreplace_b(items: list[B], x: B):
    b = heapq.heapreplace(items, x)
    b.run()


def use_heappushpop_assigned():
    xs = [A()]
    a = heappushpop(xs, A())
    a.run()
    ys = [B()]
    b = heapq.heapreplace(ys, B())
    b.run()


def use_heappushpop_walrus(items: list[A], x: A):
    if (a := heappushpop(items, x)):
        a.run()


def use_heapreplace_walrus_mod(items: list[A], x: A):
    if (a := heapq.heapreplace(items, x)):
        a.run()
