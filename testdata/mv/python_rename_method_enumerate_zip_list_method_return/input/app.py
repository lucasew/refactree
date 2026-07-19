from operator import itemgetter
import random
import heapq
from itertools import product, pairwise, zip_longest


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_enum_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(enumerate([ba.get()]))[0][1].run()
        + list(enumerate([bb.get()]))[0][1].run()
    )


def use_enum_assign(ba: BoxA, bb: BoxB) -> int:
    xa = list(enumerate([ba.get()]))[0][1]
    xb = list(enumerate([bb.get()]))[0][1]
    return xa.run() + xb.run()


def use_enum_pair(ba: BoxA, bb: BoxB) -> int:
    pair_a = list(enumerate([ba.get()]))[0]
    pair_b = list(enumerate([bb.get()]))[0]
    return pair_a[1].run() + pair_b[1].run()


def use_enum_pairs_local(ba: BoxA, bb: BoxB) -> int:
    pairs_a = list(enumerate([ba.get()]))
    pairs_b = list(enumerate([bb.get()]))
    xa = pairs_a[0][1]
    xb = pairs_b[0][1]
    return xa.run() + xb.run()


def use_enum_unpack(ba: BoxA, bb: BoxB) -> int:
    i, xa = list(enumerate([ba.get()]))[0]
    j, xb = list(enumerate([bb.get()]))[0]
    return xa.run() + xb.run()


def use_zip_direct(ba: BoxA, bb: BoxB) -> int:
    return (
        list(zip([ba.get()], [0]))[0][0].run()
        + list(zip([bb.get()], [0]))[0][0].run()
    )


def use_zip_assign(ba: BoxA, bb: BoxB) -> int:
    xa = list(zip([ba.get()], [0]))[0][0]
    xb = list(zip([bb.get()], [0]))[0][0]
    return xa.run() + xb.run()


def use_zip_unpack(ba: BoxA, bb: BoxB) -> int:
    xa, _ = list(zip([ba.get()], [0]))[0]
    xb, _ = list(zip([bb.get()], [0]))[0]
    return xa.run() + xb.run()


def use_next_enum_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        next(enumerate([ba.get()]))[1].run()
        + next(enumerate([bb.get()]))[1].run()
    )


def use_tuple_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        tuple(zip([ba.get()], [0]))[0][0].run()
        + tuple(zip([bb.get()], [0]))[0][0].run()
    )


def use_min_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        min(list(zip([ba.get()], [0])))[0].run()
        + min(list(zip([bb.get()], [0])))[0].run()
    )


def use_max_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        max(list(zip([ba.get()], [0])))[0].run()
        + max(list(zip([bb.get()], [0])))[0].run()
    )


def use_choice_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        random.choice(list(zip([ba.get()], [0])))[0].run()
        + random.choice(list(zip([bb.get()], [0])))[0].run()
    )


def use_heappop_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        heapq.heappop(list(zip([ba.get()], [0])))[0].run()
        + heapq.heappop(list(zip([bb.get()], [0])))[0].run()
    )


def use_pop_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        list(zip([ba.get()], [0])).pop()[0].run()
        + list(zip([bb.get()], [0])).pop()[0].run()
    )


def use_itemgetter_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        itemgetter(0)(list(zip([ba.get()], [0])))[0].run()
        + itemgetter(0)(list(zip([bb.get()], [0])))[0].run()
    )


def use_product(ba: BoxA, bb: BoxB) -> int:
    return (
        list(product([ba.get()], [0]))[0][0].run()
        + list(product([bb.get()], [0]))[0][0].run()
    )


def use_pairwise(ba: BoxA, bb: BoxB) -> int:
    return (
        list(pairwise([ba.get(), ba.get()]))[0][0].run()
        + list(pairwise([bb.get(), bb.get()]))[0][0].run()
    )


def use_zip_longest(ba: BoxA, bb: BoxB) -> int:
    return (
        list(zip_longest([ba.get()], [0]))[0][0].run()
        + list(zip_longest([bb.get()], [0]))[0][0].run()
    )


def use_reversed_zip(ba: BoxA, bb: BoxB) -> int:
    return (
        list(reversed(list(zip([ba.get()], [0]))))[0][0].run()
        + list(reversed(list(zip([bb.get()], [0]))))[0][0].run()
    )


def use_walrus(ba: BoxA, bb: BoxB) -> int:
    if (xa := list(enumerate([ba.get()]))[0][1]) and (
        xb := list(enumerate([bb.get()]))[0][1]
    ):
        return xa.run() + xb.run()
    return 0


def use_class() -> int:
    return (
        list(enumerate([A()]))[0][1].run()
        + list(enumerate([B()]))[0][1].run()
        + list(zip([A()], [0]))[0][0].run()
        + list(zip([B()], [0]))[0][0].run()
        + min(list(zip([A()], [0])))[0].run()
        + min(list(zip([B()], [0])))[0].run()
    )


def use_preserves_b(bb: BoxB) -> int:
    return (
        list(enumerate([bb.get()]))[0][1].run()
        + list(zip([bb.get()], [0]))[0][0].run()
        + next(enumerate([bb.get()]))[1].run()
        + min(list(zip([bb.get()], [0])))[0].run()
        + list(product([bb.get()], [0]))[0][0].run()
    )
