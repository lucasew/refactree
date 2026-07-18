import bisect
from bisect import bisect_left, bisect_right, bisect, insort_left, insort_right, insort


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_bisect_left(items: list[A], needle: A):
    bisect.bisect_left(items, needle, key=lambda p: p.execute())


def use_bisect_right(items: list[A], needle: A):
    bisect.bisect_right(items, needle, key=lambda q: q.execute())


def use_bisect(items: list[A], needle: A):
    bisect.bisect(items, needle, key=lambda r: r.execute())


def use_bisect_left_bare(items: list[A], needle: A):
    bisect_left(items, needle, key=lambda s: s.execute())


def use_bisect_right_bare(items: list[A], needle: A):
    bisect_right(items, needle, key=lambda t: t.execute())


def use_bisect_bare(items: list[A], needle: A):
    bisect(items, needle, key=lambda u: u.execute())


def use_insort(items: list[A], needle: A):
    bisect.insort_left(items, needle, key=lambda v: v.execute())
    bisect.insort_right(items, needle, key=lambda w: w.execute())
    bisect.insort(items, needle, key=lambda aa: aa.execute())


def use_insort_bare(items: list[A], needle: A):
    insort_left(items, needle, key=lambda ab: ab.execute())
    insort_right(items, needle, key=lambda ac: ac.execute())
    insort(items, needle, key=lambda ad: ad.execute())


def use_bisect_b(items: list[B], needle: B):
    bisect.bisect_left(items, needle, key=lambda be: be.run())
    bisect_right(items, needle, key=lambda bf: bf.run())


def use_bisect_assigned():
    xs = [A()]
    bisect.bisect_left(xs, A(), key=lambda ag: ag.execute())
    ys = [B()]
    bisect.bisect_right(ys, B(), key=lambda bh: bh.run())


def use_bisect_literal():
    bisect_left([A()], A(), key=lambda ai: ai.execute())
    bisect.insort([B()], B(), key=lambda bj: bj.run())


def use_bisect_wrapper(items: list[A], needle: A):
    bisect.bisect_left(list(items), needle, key=lambda ak: ak.execute())
