from operator import itemgetter
from functools import partial
import operator


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


def use_ig_list(ba: BoxA, bb: BoxB) -> int:
    return itemgetter(0)([ba.get()]).run() + itemgetter(0)([bb.get()]).run()


def use_ig_dict(ba: BoxA, bb: BoxB) -> int:
    return itemgetter("k")({"k": ba.get()}).run() + itemgetter("k")({"k": bb.get()}).run()


def use_ig_op(ba: BoxA, bb: BoxB) -> int:
    return operator.itemgetter(0)([ba.get()]).run() + operator.itemgetter(0)([bb.get()]).run()


def use_ig_assign(ba: BoxA, bb: BoxB) -> int:
    xa = itemgetter(0)([ba.get()])
    xb = itemgetter(0)([bb.get()])
    return xa.run() + xb.run()


def use_partial(ba: BoxA, bb: BoxB) -> int:
    return partial(ba.get)().run() + partial(bb.get)().run()


def use_partial_assign(ba: BoxA, bb: BoxB) -> int:
    pa = partial(ba.get)
    pb = partial(bb.get)
    return pa().run() + pb().run()


def use_class() -> int:
    return (
        itemgetter(0)([A()]).run()
        + itemgetter(0)([B()]).run()
        + partial(A)().run()
        + partial(B)().run()
    )


def use_preserves_b(bb: BoxB) -> int:
    return (
        itemgetter(0)([bb.get()]).run()
        + itemgetter("k")({"k": bb.get()}).run()
        + partial(bb.get)().run()
    )
