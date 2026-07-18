import heapq
from heapq import nlargest, nsmallest


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_nlargest_pos(items: list[A]):
    nlargest(1, items, lambda x: x.run())


def use_nlargest_pos_mod(items: list[A]):
    heapq.nlargest(1, items, lambda p: p.run())


def use_nlargest_pos_b(items: list[B]):
    nlargest(1, items, lambda y: y.run())


def use_nsmallest_pos(items: list[A]):
    nsmallest(1, items, lambda q: q.run())


def use_nsmallest_pos_mod(items: list[A]):
    heapq.nsmallest(1, items, lambda r: r.run())


def use_nsmallest_pos_b(items: list[B]):
    nsmallest(1, items, lambda z: z.run())


def use_nlargest_pos_assigned():
    xs = [A()]
    nlargest(1, xs, lambda s: s.run())
    ys = [B()]
    nlargest(1, ys, lambda t: t.run())


def use_nlargest_pos_literal():
    nlargest(1, [A()], lambda u: u.run())
    heapq.nsmallest(1, [B()], lambda v: v.run())


def use_nlargest_pos_wrapper(items: list[A]):
    nlargest(1, list(items), lambda w: w.run())
