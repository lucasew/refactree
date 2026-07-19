from collections import Counter
import collections


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_counter_for(items: list[A]):
    for a in Counter(items):
        a.execute()


def use_counter_for_b(items: list[B]):
    for b in Counter(items):
        b.run()


def use_collections_counter_for(items: list[A]):
    for a in collections.Counter(items):
        a.execute()


def use_collections_counter_for_b(items: list[B]):
    for b in collections.Counter(items):
        b.run()


def use_counter_assign(items: list[A]):
    c = Counter(items)
    for a in c:
        a.execute()


def use_counter_assign_b(items: list[B]):
    c = Counter(items)
    for b in c:
        b.run()


def use_counter_elements(items: list[A]):
    for a in Counter(items).elements():
        a.execute()


def use_counter_elements_b(items: list[B]):
    for b in Counter(items).elements():
        b.run()


def use_counter_elements_assign(items: list[A]):
    c = Counter(items)
    for a in c.elements():
        a.execute()


def use_counter_literal():
    for a in Counter([A()]):
        a.execute()
    for b in Counter([B()]):
        b.run()


def use_counter_nested(items: list[A]):
    for a in list(Counter(items)):
        a.execute()
