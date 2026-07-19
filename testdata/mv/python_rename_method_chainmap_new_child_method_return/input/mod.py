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
def use_class_nc_nested_sub() -> int:
    return (
        ChainMap({"k": [A()]}).new_child()["k"][0].run()
        + ChainMap({"k": [B()]}).new_child()["k"][0].run()
    )


def use_class_nc_scalar() -> int:
    return (
        ChainMap({"k": A()}).new_child()["k"].run()
        + ChainMap({"k": B()}).new_child()["k"].run()
    )


def use_class_nc_scalar_get() -> int:
    return (
        ChainMap({"k": A()}).new_child().get("k").run()
        + ChainMap({"k": B()}).new_child().get("k").run()
    )


def use_class_nc_assign_nested() -> int:
    ca = ChainMap({"k": [A()]}).new_child()
    cb = ChainMap({"k": [B()]}).new_child()
    return ca["k"][0].run() + cb["k"][0].run()


def use_class_nc_assign_scalar() -> int:
    ca = ChainMap({"k": A()}).new_child()
    cb = ChainMap({"k": B()}).new_child()
    return ca["k"].run() + cb["k"].run()


def use_class_nc_with_child() -> int:
    return (
        ChainMap({"k": [A()]}).new_child({"m": [A()]})["m"][0].run()
        + ChainMap({"k": [B()]}).new_child({"m": [B()]})["m"][0].run()
    )


def use_class_nc_od() -> int:
    return (
        ChainMap(OrderedDict(k=[A()])).new_child()["k"][0].run()
        + ChainMap(OrderedDict(k=[B()])).new_child()["k"][0].run()
    )


def use_class_coll_nc() -> int:
    return (
        collections.ChainMap({"k": [A()]}).new_child()["k"][0].run()
        + collections.ChainMap({"k": [B()]}).new_child()["k"][0].run()
    )


# Method-return under foreign same-leaf.
def use_mr_nc_nested_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]}).new_child()["k"][0].run()
        + ChainMap({"k": [bb.get()]}).new_child()["k"][0].run()
    )


def use_mr_nc_scalar(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).new_child()["k"].run()
        + ChainMap({"k": bb.get()}).new_child()["k"].run()
    )


def use_mr_nc_scalar_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).new_child().get("k").run()
        + ChainMap({"k": bb.get()}).new_child().get("k").run()
    )


def use_mr_nc_assign_nested(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": [ba.get()]}).new_child()
    cb = ChainMap({"k": [bb.get()]}).new_child()
    return ca["k"][0].run() + cb["k"][0].run()


def use_mr_nc_assign_scalar(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": ba.get()}).new_child()
    cb = ChainMap({"k": bb.get()}).new_child()
    return ca["k"].run() + cb["k"].run()


def use_mr_nc_with_child(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]}).new_child({"m": [ba.get()]})["m"][0].run()
        + ChainMap({"k": [bb.get()]}).new_child({"m": [bb.get()]})["m"][0].run()
    )


def use_mr_nc_od(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap(OrderedDict(k=[ba.get()])).new_child()["k"][0].run()
        + ChainMap(OrderedDict(k=[bb.get()])).new_child()["k"][0].run()
    )


def use_mr_coll_nc(ba: BoxA, bb: BoxB) -> int:
    return (
        collections.ChainMap({"k": [ba.get()]}).new_child()["k"][0].run()
        + collections.ChainMap({"k": [bb.get()]}).new_child()["k"][0].run()
    )


# Plain ChainMap nested (no new_child) — already solid regression.
def use_mr_plain_nested(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": [ba.get()]})["k"][0].run()
        + ChainMap({"k": [bb.get()]})["k"][0].run()
    )


def use_preserves_b(bb: BoxB) -> int:
    return (
        ChainMap({"k": [bb.get()]}).new_child()["k"][0].run()
        + ChainMap({"k": bb.get()}).new_child()["k"].run()
        + ChainMap({"k": bb.get()}).new_child().get("k").run()
        + ChainMap({"k": [bb.get()]}).new_child({"m": [bb.get()]})["m"][0].run()
        + ChainMap(OrderedDict(k=[bb.get()])).new_child()["k"][0].run()
        + collections.ChainMap({"k": [bb.get()]}).new_child()["k"][0].run()
        + B().run()
    )
