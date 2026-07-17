from collections import deque
import collections


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_deque_for(items: list[A]):
    for a in deque(items):
        a.run()


def use_deque_for_b(items: list[B]):
    for b in deque(items):
        b.run()


def use_collections_deque_for(items: list[A]):
    for a in collections.deque(items):
        a.run()


def use_deque_assign(items: list[A]):
    xs = deque(items)
    a = xs.popleft()
    a.run()


def use_deque_assign_b(items: list[B]):
    ys = deque(items)
    b = ys.popleft()
    b.run()


def use_deque_literal():
    for a in deque([A()]):
        a.run()
    for b in deque([B()]):
        b.run()


def use_deque_nested(items: list[A]):
    for a in list(deque(items)):
        a.run()
