import itertools
from itertools import cycle


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_cycle(items: list[A]):
    for a in itertools.cycle(items):
        a.run()


def use_cycle_imported(items: list[A]):
    for a in cycle(items):
        a.run()


def use_cycle_b(items: list[B]):
    for b in cycle(items):
        b.run()


def use_cycle_literal():
    for a in cycle([A()]):
        a.run()
    for b in itertools.cycle([B()]):
        b.run()


def use_cycle_assigned():
    xs = [A()]
    for a in cycle(xs):
        a.run()
    ys = [B()]
    for b in itertools.cycle(ys):
        b.run()


def use_cycle_nested(items: list[A]):
    for a in list(cycle(items)):
        a.run()


def use_cycle_bind(items: list[A]):
    it = cycle(items)
    for a in it:
        a.run()
