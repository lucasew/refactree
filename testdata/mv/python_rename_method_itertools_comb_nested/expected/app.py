import itertools
from itertools import combinations, permutations, combinations_with_replacement


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_comb_nested(items: list[A]):
    for combo in itertools.combinations(items, 2):
        for a in combo:
            a.execute()


def use_comb_nested_imported(items: list[A]):
    for combo in combinations(items, 2):
        for a in combo:
            a.execute()


def use_perm_nested(items: list[A]):
    for combo in itertools.permutations(items, 2):
        for a in combo:
            a.execute()


def use_perm_nested_imported(items: list[A]):
    for combo in permutations(items, 2):
        for a in combo:
            a.execute()


def use_cwr_nested(items: list[A]):
    for combo in combinations_with_replacement(items, 2):
        for a in combo:
            a.execute()
    for combo in itertools.combinations_with_replacement(items, 2):
        a = next(combo)
        a.execute()


def use_comb_nested_b(items: list[B]):
    for combo in combinations(items, 2):
        for b in combo:
            b.run()


def use_comb_nested_literal():
    for combo in combinations([A(), A()], 2):
        for a in combo:
            a.execute()
    for combo in itertools.permutations([B(), B()], 2):
        for b in combo:
            b.run()


def use_comb_nested_assigned():
    xs = [A(), A()]
    for combo in combinations(xs, 2):
        for a in list(combo):
            a.execute()
    ys = [B(), B()]
    for combo in itertools.combinations(ys, 2):
        for b in combo:
            b.run()


def use_comb_nested_preserves_b(items: list[B]):
    for combo in combinations(items, 2):
        for b in combo:
            b.run()
