from itertools import tee
import itertools


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    item: A

    def __init__(self, item: A):
        self.item = item

    def get(self) -> A:
        return self.item


class BoxB:
    item: B

    def __init__(self, item: B):
        self.item = item

    def get(self) -> B:
        return self.item


def use_tee_for(ba: BoxA, bb: BoxB) -> int:
    ita1, ita2 = tee([ba.get()])
    n = 0
    for xa in ita1:
        n += xa.run()
    itb1, itb2 = itertools.tee([bb.get()])
    for xb in itb1:
        n += xb.run()
    return n


def use_tee_next(ba: BoxA, bb: BoxB) -> int:
    ita1, ita2 = tee([ba.get()])
    itb1, itb2 = itertools.tee([bb.get()])
    return next(ita1).run() + next(itb1).run()


def use_tee_assign(ba: BoxA, bb: BoxB) -> int:
    ita1, ita2 = tee([ba.get()])
    xa = next(ita1)
    itb1, itb2 = itertools.tee([bb.get()])
    xb = next(itb1)
    return xa.run() + xb.run()


def use_tee_n(ba: BoxA, bb: BoxB) -> int:
    a1, a2, a3 = tee([ba.get()], 3)
    b1, b2, b3 = itertools.tee([bb.get()], 3)
    return next(a1).run() + next(a2).run() + next(a3).run() + next(b1).run()


def use_tee_dunder(ba: BoxA, bb: BoxB) -> int:
    ita1, ita2 = tee([ba.get()])
    itb1, itb2 = itertools.tee([bb.get()])
    return ita1.__next__().run() + itb1.__next__().run()


def use_preserves_b(bb: BoxB) -> int:
    it_only_b1, it_only_b2 = tee([bb.get()])
    return next(it_only_b1).run()
