import itertools
from itertools import batched


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_batched_nested(items: list[A]):
    for batch in itertools.batched(items, 2):
        for a in batch:
            a.run()


def use_batched_nested_imported(items: list[A]):
    for batch in batched(items, 2):
        for a in batch:
            a.run()


def use_batched_unpack(items: list[A]):
    for a, nxt in itertools.batched(items, 2):
        a.run()
        nxt.run()


def use_batched_unpack_imported(items: list[A]):
    for a, nxt in batched(items, 2):
        a.run()
        nxt.run()


def use_batched_b(items: list[B]):
    for batch in batched(items, 2):
        for b in batch:
            b.run()
    for b1, b2 in batched(items, 2):
        b1.run()
        b2.run()


def use_batched_literal():
    for batch in batched([A(), A()], 2):
        for a in batch:
            a.run()
    for a, nxt in itertools.batched([A(), A()], 2):
        a.run()
        nxt.run()
    for batch in itertools.batched([B(), B()], 2):
        for b in batch:
            b.run()


def use_batched_assigned():
    xs = [A(), A()]
    for batch in batched(xs, 2):
        for a in batch:
            a.run()
    for a, nxt in itertools.batched(xs, 2):
        a.run()
        nxt.run()
    ys = [B(), B()]
    for batch in itertools.batched(ys, 2):
        for b in batch:
            b.run()


def use_batched_list_next(items: list[A]):
    for batch in batched(items, 2):
        for a in list(batch):
            a.run()
    for batch in itertools.batched(items, 2):
        a = next(batch)
        a.run()


def use_batched_strict(items: list[A]):
    for batch in batched(items, 2, strict=False):
        for a in batch:
            a.run()


def use_batched_preserves_b(items: list[B]):
    for batch in batched(items, 2):
        for b in batch:
            b.run()
