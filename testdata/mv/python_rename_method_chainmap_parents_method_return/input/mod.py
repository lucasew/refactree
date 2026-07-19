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


# Class regressions — already solid (typed locals + Class() multi-map).
def use_class_parents_typed(da: dict[str, A], ea: dict[str, A], db: dict[str, B], eb: dict[str, B]) -> int:
    return (
        ChainMap(da, ea).parents["k"].run()
        + ChainMap(db, eb).parents["k"].run()
    )


def use_class_parents_assign(da: dict[str, A], ea: dict[str, A], db: dict[str, B], eb: dict[str, B]) -> int:
    pa = ChainMap(da, ea).parents
    pb = ChainMap(db, eb).parents
    return pa["k"].run() + pb["k"].run()


def use_class_parents_lit() -> int:
    return (
        ChainMap({"k": A()}, {"j": A()}).parents["k"].run()
        + ChainMap({"k": B()}, {"j": B()}).parents["k"].run()
    )


def use_class_parents_get() -> int:
    return (
        ChainMap({"k": A()}, {"j": A()}).parents.get("k").run()
        + ChainMap({"k": B()}, {"j": B()}).parents.get("k").run()
    )


def use_class_parents_values() -> int:
    return (
        list(ChainMap({"k": A()}, {"j": A()}).parents.values())[0].run()
        + list(ChainMap({"k": B()}, {"j": B()}).parents.values())[0].run()
    )


def use_class_coll_parents() -> int:
    return (
        collections.ChainMap({"k": A()}, {"j": A()}).parents["k"].run()
        + collections.ChainMap({"k": B()}, {"j": B()}).parents["k"].run()
    )


def use_class_od_parents() -> int:
    return (
        ChainMap(OrderedDict(k=A()), OrderedDict(k=A())).parents["k"].run()
        + ChainMap(OrderedDict(k=B()), OrderedDict(k=B())).parents["k"].run()
    )


# Method-return under foreign same-leaf.
def use_mr_parents_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}, {"j": ba.get()}).parents["k"].run()
        + ChainMap({"k": bb.get()}, {"j": bb.get()}).parents["k"].run()
    )


def use_mr_parents_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}, {"j": ba.get()}).parents.get("k").run()
        + ChainMap({"k": bb.get()}, {"j": bb.get()}).parents.get("k").run()
    )


def use_mr_parents_assign(ba: BoxA, bb: BoxB) -> int:
    pa = ChainMap({"k": ba.get()}, {"j": ba.get()}).parents
    pb = ChainMap({"k": bb.get()}, {"j": bb.get()}).parents
    return pa["k"].run() + pb["k"].run()


def use_mr_coll_parents(ba: BoxA, bb: BoxB) -> int:
    return (
        collections.ChainMap({"k": ba.get()}, {"j": ba.get()}).parents["k"].run()
        + collections.ChainMap({"k": bb.get()}, {"j": bb.get()}).parents["k"].run()
    )


def use_mr_od_parents(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap(OrderedDict(k=ba.get()), OrderedDict(k=ba.get())).parents["k"].run()
        + ChainMap(OrderedDict(k=bb.get()), OrderedDict(k=bb.get())).parents["k"].run()
    )


def use_mr_parents_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(ChainMap({"k": ba.get()}, {"j": ba.get()}).parents.values())[0].run()
        + list(ChainMap({"k": bb.get()}, {"j": bb.get()}).parents.values())[0].run()
    )


# Plain ChainMap subscript regression (no parents).
def use_mr_cm_plain(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()})["k"].run()
        + ChainMap({"k": bb.get()})["k"].run()
    )


# Preserves B.
def use_preserves_b(bb: BoxB) -> int:
    return ChainMap({"k": bb.get()}, {"j": bb.get()}).parents["k"].run()
