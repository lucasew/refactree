import itertools
from itertools import groupby


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_groups_next_unpack(items: list[A]):
    groups = groupby(items)
    _, ga = next(groups)
    a = next(ga)
    a.execute()


def use_groups_itertools(items: list[A]):
    groups = itertools.groupby(items)
    _, ga = next(groups)
    a = next(ga)
    a.execute()


def use_groups_for(items: list[A]):
    groups = groupby(items)
    _, ga = next(groups)
    for a in ga:
        a.execute()


def use_groups_for_loop(items: list[A]):
    groups = groupby(items)
    for k, g in groups:
        for a in g:
            a.execute()


def use_groups_list(items: list[A]):
    groups = groupby(items)
    _, ga = next(groups)
    for a in list(ga):
        a.execute()


def use_groups_b(items: list[B]):
    groups = groupby(items)
    _, gb = next(groups)
    b = next(gb)
    b.run()


def use_groups_literal():
    groups = groupby([A(), A()])
    _, ga = next(groups)
    a = next(ga)
    a.execute()
    groupsb = itertools.groupby([B(), B()])
    _, gb = next(groupsb)
    b = next(gb)
    b.run()


def use_groups_key(items: list[A]):
    groups = groupby(items, key=lambda x: 0)
    k, ga = next(groups)
    a = next(ga)
    a.execute()


def use_groups_alias(items: list[A]):
    groups = groupby(items)
    gs = groups
    _, ga = next(gs)
    a = next(ga)
    a.execute()


def use_groups_next_sub(items: list[A]):
    groups = groupby(items)
    ga = next(groups)[1]
    a = next(ga)
    a.execute()


def use_next_groupby_sub(items: list[A]):
    ga = next(groupby(items))[1]
    a = next(ga)
    a.execute()


def use_next_groupby_sub_b(items: list[B]):
    ga = next(groupby(items))[1]
    b = next(ga)
    b.run()


def use_iter_groupby(items: list[A]):
    groups = iter(groupby(items))
    _, ga = next(groups)
    a = next(ga)
    a.execute()


def use_iter_groupby_b(items: list[B]):
    groups = iter(groupby(items))
    _, gb = next(groups)
    b = next(gb)
    b.run()
