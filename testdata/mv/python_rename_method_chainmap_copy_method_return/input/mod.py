from collections import ChainMap, OrderedDict
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A) -> None:
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B) -> None:
        self.b = b

    def get(self) -> B:
        return self.b


# Class regressions — already solid.
def use_class_cm_copy_sub() -> int:
    return (
        ChainMap({"k": A()}).copy()["k"].run()
        + ChainMap({"k": B()}).copy()["k"].run()
    )


def use_class_cm_copy_get() -> int:
    return (
        ChainMap({"k": A()}).copy().get("k").run()
        + ChainMap({"k": B()}).copy().get("k").run()
    )


def use_class_cm_copy_assign() -> int:
    ca = ChainMap({"k": A()}).copy()
    cb = ChainMap({"k": B()}).copy()
    return ca["k"].run() + cb["k"].run()


def use_class_cm_copy_multi() -> int:
    return (
        ChainMap({"k": A()}, {"j": A()}).copy()["k"].run()
        + ChainMap({"k": B()}, {"j": B()}).copy()["k"].run()
    )


def use_class_coll_cm_copy() -> int:
    return (
        collections.ChainMap({"k": A()}).copy()["k"].run()
        + collections.ChainMap({"k": B()}).copy()["k"].run()
    )


def use_class_od_cm_copy() -> int:
    return (
        ChainMap(OrderedDict(k=A())).copy()["k"].run()
        + ChainMap(OrderedDict(k=B())).copy()["k"].run()
    )


def use_class_cm_plain() -> int:
    return (
        ChainMap({"k": A()})["k"].run()
        + ChainMap({"k": B()})["k"].run()
    )


# Method-return under foreign same-leaf.
def use_mr_cm_copy_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).copy()["k"].run()
        + ChainMap({"k": bb.get()}).copy()["k"].run()
    )


def use_mr_cm_copy_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).copy().get("k").run()
        + ChainMap({"k": bb.get()}).copy().get("k").run()
    )


def use_mr_cm_copy_assign(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": ba.get()}).copy()
    cb = ChainMap({"k": bb.get()}).copy()
    return ca["k"].run() + cb["k"].run()


def use_mr_cm_copy_multi(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}, {"j": ba.get()}).copy()["k"].run()
        + ChainMap({"k": bb.get()}, {"j": bb.get()}).copy()["k"].run()
    )


def use_mr_coll_cm_copy(ba: BoxA, bb: BoxB) -> int:
    return (
        collections.ChainMap({"k": ba.get()}).copy()["k"].run()
        + collections.ChainMap({"k": bb.get()}).copy()["k"].run()
    )


def use_mr_od_cm_copy(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap(OrderedDict(k=ba.get())).copy()["k"].run()
        + ChainMap(OrderedDict(k=bb.get())).copy()["k"].run()
    )


def use_mr_cm_plain(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()})["k"].run()
        + ChainMap({"k": bb.get()})["k"].run()
    )


# Preserves B.
def use_preserves_b(bb: BoxB) -> int:
    return ChainMap({"k": bb.get()}).copy()["k"].run()
