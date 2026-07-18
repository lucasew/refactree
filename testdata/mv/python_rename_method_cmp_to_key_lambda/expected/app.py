from functools import cmp_to_key
import functools


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_sorted_cmp(items: list[A]):
    sorted(items, key=cmp_to_key(lambda ca, cb: ca.execute() - cb.execute()))


def use_sorted_cmp_mod(items: list[A]):
    sorted(items, key=functools.cmp_to_key(lambda da, db: da.execute() - db.execute()))


def use_min_cmp(items: list[A]):
    min(items, key=cmp_to_key(lambda ea, eb: ea.execute() - eb.execute()))


def use_max_cmp(items: list[A]):
    max(items, key=cmp_to_key(lambda fa, fb: fa.execute() - fb.execute()))


def use_sort_cmp(items: list[A]):
    items.sort(key=cmp_to_key(lambda ga, gb: ga.execute() - gb.execute()))


def use_sorted_cmp_b(items: list[B]):
    sorted(items, key=cmp_to_key(lambda ha, hb: ha.run() - hb.run()))


def use_sorted_cmp_assigned():
    xs = [A()]
    sorted(xs, key=cmp_to_key(lambda ia, ib: ia.execute() - ib.execute()))
    ys = [B()]
    sorted(ys, key=cmp_to_key(lambda ja, jb: ja.run() - jb.run()))


def use_sorted_cmp_literal():
    sorted([A()], key=cmp_to_key(lambda ka, kb: ka.execute() - kb.execute()))
    sorted([B()], key=functools.cmp_to_key(lambda la, lb: la.run() - lb.run()))


def use_sorted_cmp_wrapper(items: list[A]):
    sorted(list(items), key=cmp_to_key(lambda ma, mb: ma.execute() - mb.execute()))
