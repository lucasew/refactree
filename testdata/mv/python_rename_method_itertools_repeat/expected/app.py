import itertools
from itertools import repeat


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_repeat(item: A):
    for a in itertools.repeat(item):
        a.execute()


def use_repeat_imported(item: A):
    for a in repeat(item):
        a.execute()


def use_repeat_times(item: A):
    for a in itertools.repeat(item, 3):
        a.execute()


def use_repeat_b(item: B):
    for b in repeat(item):
        b.run()


def use_repeat_literal():
    for a in repeat(A()):
        a.execute()
    for b in itertools.repeat(B()):
        b.run()


def use_repeat_assigned():
    x = A()
    for a in repeat(x):
        a.execute()
    y = B()
    for b in itertools.repeat(y):
        b.run()


def use_repeat_nested(item: A):
    for a in list(repeat(item)):
        a.execute()


def use_repeat_bind(item: A):
    it = repeat(item)
    for a in it:
        a.execute()
