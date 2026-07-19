import functools
import itertools
from functools import reduce
from itertools import accumulate


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_reduce_bi(items: list[A]):
    a = reduce(lambda x, y: x if x.execute() else y, items)
    a.execute()


def use_reduce_bi_b(items: list[B]):
    b = reduce(lambda p, q: p if p.run() else q, items)
    b.run()


def use_functools_reduce_bi(items: list[A]):
    a = functools.reduce(lambda x, y: y if y.execute() else x, items)
    a.execute()


def use_functools_reduce_bi_b(items: list[B]):
    b = functools.reduce(lambda p, q: q if q.run() else p, items)
    b.run()


def use_reduce_bi_both(items: list[A]):
    a = reduce(lambda x, y: x if x.execute() > y.execute() else y, items)
    a.execute()


def use_reduce_init_bi(items: list[A], da: A):
    a = reduce(lambda x, y: x if x.execute() else y, items, da)
    a.execute()


def use_accumulate_bi(items: list[A]):
    for a in accumulate(items, lambda x, y: x if x.execute() else y):
        a.execute()


def use_accumulate_bi_mod(items: list[A]):
    for a in itertools.accumulate(items, lambda x, y: y if y.execute() else x):
        a.execute()


def use_accumulate_bi_b(items: list[B]):
    for b in accumulate(items, lambda p, q: p if p.run() else q):
        b.run()


def use_accumulate_func_kw(items: list[A]):
    for a in accumulate(items, func=lambda x, y: x if x.execute() else y):
        a.execute()


def use_accumulate_func_kw_b(items: list[B]):
    for b in itertools.accumulate(items, func=lambda p, q: p if p.run() else q):
        b.run()


def use_reduce_assigned():
    xs = [A()]
    a = reduce(lambda x, y: x if x.execute() else y, xs)
    a.execute()
    ys = [B()]
    b = reduce(lambda p, q: p if p.run() else q, ys)
    b.run()


def use_reduce_literal():
    a = reduce(lambda x, y: x if x.execute() else y, [A()])
    a.execute()
    b = functools.reduce(lambda p, q: p if p.run() else q, [B()])
    b.run()


def use_reduce_wrapper(items: list[A]):
    a = reduce(lambda x, y: x if x.execute() else y, list(items))
    a.execute()
