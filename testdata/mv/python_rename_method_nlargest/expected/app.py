import heapq
from heapq import nlargest, nsmallest


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_nlargest(items: list[A]):
    for a in heapq.nlargest(3, items):
        a.execute()


def use_nsmallest(items: list[A]):
    for a in heapq.nsmallest(2, items):
        a.execute()


def use_nlargest_imported(items: list[A]):
    for a in nlargest(3, items):
        a.execute()


def use_nsmallest_imported(items: list[A]):
    for a in nsmallest(2, items):
        a.execute()


def use_nlargest_b(items: list[B]):
    for b in nlargest(1, items):
        b.run()


def use_nsmallest_b(items: list[B]):
    for b in heapq.nsmallest(1, items):
        b.run()


def use_nlargest_literal():
    for a in nlargest(1, [A()]):
        a.execute()
    for b in heapq.nsmallest(1, [B()]):
        b.run()


def use_nlargest_assigned():
    xs = [A()]
    for a in nlargest(2, xs):
        a.execute()
    ys = [B()]
    for b in heapq.nsmallest(2, ys):
        b.run()


def use_nlargest_bind(items: list[A]):
    top = nlargest(3, items)
    for a in top:
        a.execute()


def use_nsmallest_bind(items: list[A]):
    bot = heapq.nsmallest(2, items)
    for a in bot:
        a.execute()


def use_nlargest_nested(items: list[A]):
    for a in list(nlargest(3, items)):
        a.execute()


def use_nlargest_key(items: list[A]):
    for a in nlargest(3, items, key=lambda x: 0):
        a.execute()
