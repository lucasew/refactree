import itertools
from itertools import takewhile, dropwhile, filterfalse, compress


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_takewhile(items: list[A]):
    for a in itertools.takewhile(lambda x: True, items):
        a.run()


def use_takewhile_imported(items: list[A]):
    for a in takewhile(lambda x: True, items):
        a.run()


def use_dropwhile(items: list[A]):
    for a in itertools.dropwhile(lambda x: False, items):
        a.run()


def use_dropwhile_imported(items: list[A]):
    for a in dropwhile(lambda x: False, items):
        a.run()


def use_filterfalse(items: list[A]):
    for a in itertools.filterfalse(None, items):
        a.run()


def use_filterfalse_imported(items: list[A]):
    for a in filterfalse(None, items):
        a.run()


def use_compress(items: list[A], selectors: list[bool]):
    for a in itertools.compress(items, selectors):
        a.run()


def use_compress_imported(items: list[A], selectors: list[bool]):
    for a in compress(items, selectors):
        a.run()


def use_takewhile_b(items: list[B]):
    for b in takewhile(lambda x: True, items):
        b.run()


def use_filterfalse_b(items: list[B]):
    for b in filterfalse(None, items):
        b.run()


def use_compress_b(items: list[B], selectors: list[bool]):
    for b in compress(items, selectors):
        b.run()


def use_takewhile_literal():
    for a in takewhile(lambda x: True, [A()]):
        a.run()
    for b in itertools.dropwhile(lambda x: False, [B()]):
        b.run()


def use_compress_literal(selectors: list[bool]):
    for a in compress([A()], selectors):
        a.run()
    for b in itertools.compress([B()], selectors):
        b.run()


def use_takewhile_assigned():
    xs = [A()]
    for a in takewhile(lambda x: True, xs):
        a.run()
    ys = [B()]
    for b in itertools.filterfalse(None, ys):
        b.run()


def use_takewhile_nested(items: list[A]):
    for a in list(takewhile(lambda x: True, items)):
        a.run()


def use_filterfalse_nested(items: list[A]):
    for a in list(itertools.filterfalse(None, items)):
        a.run()


def use_compress_nested(items: list[A], selectors: list[bool]):
    for a in list(compress(items, selectors)):
        a.run()


def use_takewhile_bind(items: list[A]):
    it = takewhile(lambda x: True, items)
    for a in it:
        a.run()


def use_compress_bind(items: list[A], selectors: list[bool]):
    it = itertools.compress(items, selectors)
    for a in it:
        a.run()
