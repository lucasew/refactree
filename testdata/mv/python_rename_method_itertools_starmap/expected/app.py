import itertools
from itertools import starmap


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_starmap(pairs: list[tuple]):
    for a in itertools.starmap(A, pairs):
        a.execute()


def use_starmap_imported(pairs: list[tuple]):
    for a in starmap(A, pairs):
        a.execute()


def use_starmap_b(pairs: list[tuple]):
    for b in starmap(B, pairs):
        b.run()


def use_starmap_nested(pairs: list[tuple]):
    for a in list(starmap(A, pairs)):
        a.execute()


def use_starmap_bind(pairs: list[tuple]):
    it = itertools.starmap(A, pairs)
    for a in it:
        a.execute()


def use_starmap_bind_b(pairs: list[tuple]):
    it = starmap(B, pairs)
    for b in it:
        b.run()
