import heapq
from heapq import merge


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_merge_key(xs: list[A], ys: list[A]):
    merge(xs, ys, key=lambda x: x.run())


def use_merge_key_mod(xs: list[A], ys: list[A]):
    heapq.merge(xs, ys, key=lambda p: p.run())


def use_merge_key_b(xs: list[B], ys: list[B]):
    merge(xs, ys, key=lambda y: y.run())


def use_merge_key_mod_b(xs: list[B], ys: list[B]):
    heapq.merge(xs, ys, key=lambda y: y.run())


def use_merge_key_for(xs: list[A], ys: list[A]):
    for a in merge(xs, ys, key=lambda x: x.run()):
        a.run()


def use_merge_key_for_b(xs: list[B], ys: list[B]):
    for b in heapq.merge(xs, ys, key=lambda y: y.run()):
        b.run()


def use_merge_key_assigned():
    xs = [A()]
    ys = [A()]
    merge(xs, ys, key=lambda x: x.run())
    zs = [B()]
    ws = [B()]
    heapq.merge(zs, ws, key=lambda y: y.run())


def use_merge_key_literal():
    merge([A()], [A()], key=lambda x: x.run())
    heapq.merge([B()], [B()], key=lambda y: y.run())


def use_merge_key_wrapper(xs: list[A], ys: list[A]):
    merge(list(xs), list(ys), key=lambda x: x.run())


def use_merge_key_three(xs: list[A], ys: list[A], zs: list[A]):
    heapq.merge(xs, ys, zs, key=lambda x: x.run())


def use_merge_key_reverse(xs: list[A], ys: list[A]):
    merge(xs, ys, key=lambda x: x.run(), reverse=True)
