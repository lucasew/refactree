from functools import reduce
import functools


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_reduce(items: list[A]):
    a = reduce(lambda x, y: x, items)
    a.execute()


def use_reduce_b(items: list[B]):
    b = reduce(lambda x, y: x, items)
    b.run()


def use_functools_reduce(items: list[A]):
    a = functools.reduce(lambda x, y: x, items)
    a.execute()


def use_functools_reduce_b(items: list[B]):
    b = functools.reduce(lambda x, y: x, items)
    b.run()


def use_reduce_init(items: list[A], da: A):
    a = reduce(lambda x, y: x, items, da)
    a.execute()


def use_reduce_init_b(items: list[B], db: B):
    b = reduce(lambda x, y: x, items, db)
    b.run()


def use_reduce_walrus(items: list[A]):
    if (a := reduce(lambda x, y: x, items)):
        a.execute()


def use_reduce_assigned():
    xs = [A()]
    a = reduce(lambda x, y: x, xs)
    a.execute()
    ys = [B()]
    b = reduce(lambda x, y: x, ys)
    b.run()


def use_reduce_wrapper(items: list[A]):
    a = reduce(lambda x, y: x, list(items))
    a.execute()
