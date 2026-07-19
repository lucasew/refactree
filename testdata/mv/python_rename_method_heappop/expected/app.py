import heapq
from heapq import heappop


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_heappop(items: list[A]):
    a = heapq.heappop(items)
    a.execute()


def use_heappop_imported(items: list[A]):
    a = heappop(items)
    a.execute()


def use_heappop_b(items: list[B]):
    b = heapq.heappop(items)
    b.run()


def use_heappop_assigned():
    xs = [A()]
    a = heappop(xs)
    a.execute()
    ys = [B()]
    b = heapq.heappop(ys)
    b.run()


def use_heappop_walrus(items: list[A]):
    if (a := heappop(items)):
        a.execute()


def use_heappop_walrus_mod(items: list[A]):
    if (a := heapq.heappop(items)):
        a.execute()
