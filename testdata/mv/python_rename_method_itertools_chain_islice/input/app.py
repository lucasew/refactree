import itertools
from itertools import chain, islice


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_chain(items: list[A], more: list[A]):
    for a in itertools.chain(items, more):
        a.run()


def use_chain_imported(items: list[A], more: list[A]):
    for a in chain(items, more):
        a.run()


def use_chain_b(items: list[B], more: list[B]):
    for b in itertools.chain(items, more):
        b.run()


def use_chain_literal():
    for a in itertools.chain([A()], [A()]):
        a.run()
    for b in chain([B()], [B()]):
        b.run()


def use_chain_assigned():
    xs = [A()]
    ys = [A()]
    for a in chain(xs, ys):
        a.run()
    zs = [B()]
    ws = [B()]
    for b in itertools.chain(zs, ws):
        b.run()


def use_islice(items: list[A]):
    for a in itertools.islice(items, 1, 3):
        a.run()


def use_islice_imported(items: list[A]):
    for a in islice(items, 2):
        a.run()


def use_islice_b(items: list[B]):
    for b in islice(items, 0, None, 2):
        b.run()


def use_islice_nested(items: list[A]):
    for a in list(itertools.islice(items, 5)):
        a.run()


def use_chain_single(items: list[A]):
    for a in chain(items):
        a.run()
