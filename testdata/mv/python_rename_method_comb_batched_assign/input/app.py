import itertools
from itertools import batched, combinations, permutations, combinations_with_replacement


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_comb_assign(items: list[A]):
    combos = combinations(items, 2)
    for a, nxt in combos:
        a.run()
        nxt.run()


def use_comb_assign_nested(items: list[A]):
    combos = combinations(items, 2)
    for combo in combos:
        for a in combo:
            a.run()


def use_comb_assign_itertools(items: list[A]):
    combos = itertools.combinations(items, 2)
    for a, nxt in combos:
        a.run()
        nxt.run()
    combos2 = itertools.combinations(items, 2)
    for combo in combos2:
        a = next(combo)
        a.run()


def use_perm_assign(items: list[A]):
    combos = permutations(items, 2)
    for a, nxt in combos:
        a.run()
        nxt.run()
    combos2 = itertools.permutations(items, 2)
    for combo in combos2:
        for a in combo:
            a.run()


def use_cwr_assign(items: list[A]):
    combos = combinations_with_replacement(items, 2)
    for a, nxt in combos:
        a.run()
        nxt.run()


def use_batched_assign(items: list[A]):
    batches = batched(items, 2)
    for a, nxt in batches:
        a.run()
        nxt.run()


def use_batched_assign_nested(items: list[A]):
    batches = batched(items, 2)
    for batch in batches:
        for a in batch:
            a.run()


def use_batched_assign_itertools(items: list[A]):
    batches = itertools.batched(items, 2)
    for a, nxt in batches:
        a.run()
        nxt.run()
    batches2 = itertools.batched(items, 2)
    for batch in batches2:
        a = next(batch)
        a.run()


def use_comb_assign_b(items: list[B]):
    combos = combinations(items, 2)
    for x, y in combos:
        x.run()
        y.run()


def use_batched_assign_b(items: list[B]):
    batches = batched(items, 2)
    for x, y in batches:
        x.run()
    batches2 = batched(items, 2)
    for batch in batches2:
        for x in batch:
            x.run()


def use_comb_assign_literal():
    combos = combinations([A(), A()], 2)
    for a, nxt in combos:
        a.run()
    combos2 = combinations([B(), B()], 2)
    for x, y in combos2:
        x.run()


def use_batched_assign_literal():
    batches = batched([A(), A()], 2)
    for a, nxt in batches:
        a.run()
    batches2 = itertools.batched([B(), B()], 2)
    for x, y in batches2:
        x.run()


def use_comb_assign_assigned():
    xs = [A(), A()]
    combos = combinations(xs, 2)
    for a, nxt in combos:
        a.run()
    ys = [B(), B()]
    combos2 = itertools.combinations(ys, 2)
    for combo in combos2:
        for x in combo:
            x.run()


def use_batched_assign_assigned():
    xs = [A(), A()]
    batches = batched(xs, 2)
    for a, nxt in batches:
        a.run()
    ys = [B(), B()]
    batches2 = itertools.batched(ys, 2)
    for batch in batches2:
        for x in batch:
            x.run()


def use_comb_assign_preserves_b(items: list[B]):
    combos = combinations(items, 2)
    for x, y in combos:
        x.run()


def use_batched_assign_preserves_b(items: list[B]):
    batches = batched(items, 2)
    for batch in batches:
        for x in batch:
            x.run()
