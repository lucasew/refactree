import heapq
from heapq import merge


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_merge(xs: list[A], ys: list[A]):
    for a in heapq.merge(xs, ys):
        a.run()


def use_merge_imported(xs: list[A], ys: list[A]):
    for a in merge(xs, ys):
        a.run()


def use_merge_b(xs: list[B], ys: list[B]):
    for b in heapq.merge(xs, ys):
        b.run()


def use_merge_key(xs: list[A], ys: list[A]):
    for a in heapq.merge(xs, ys, key=lambda x: 0):
        a.run()


def use_merge_assigned():
    xs = [A()]
    ys = [A()]
    for a in merge(xs, ys):
        a.run()
    zs = [B()]
    ws = [B()]
    for b in heapq.merge(zs, ws):
        b.run()


def use_merge_literal():
    for a in heapq.merge([A()], [A()]):
        a.run()
    for b in merge([B()], [B()]):
        b.run()


def use_merge_comp(xs: list[A], ys: list[A]):
    return [a.run() for a in heapq.merge(xs, ys)]
