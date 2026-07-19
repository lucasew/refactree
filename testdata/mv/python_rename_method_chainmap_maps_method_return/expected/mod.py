from collections import ChainMap, OrderedDict
import collections


class A:
    def execute(self) -> int:
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
def use_class_maps_sub() -> int:
    return (
        ChainMap({"k": A()}).maps[0]["k"].execute()
        + ChainMap({"k": B()}).maps[0]["k"].run()
    )


def use_class_maps_get() -> int:
    return (
        ChainMap({"k": A()}).maps[0].get("k").execute()
        + ChainMap({"k": B()}).maps[0].get("k").run()
    )


def use_class_maps_assign() -> int:
    ca = ChainMap({"k": A()})
    cb = ChainMap({"k": B()})
    return ca.maps[0]["k"].execute() + cb.maps[0]["k"].run()


def use_class_coll_maps() -> int:
    return (
        collections.ChainMap({"k": A()}).maps[0]["k"].execute()
        + collections.ChainMap({"k": B()}).maps[0]["k"].run()
    )


def use_class_od_maps() -> int:
    return (
        ChainMap(OrderedDict(k=A())).maps[0]["k"].execute()
        + ChainMap(OrderedDict(k=B())).maps[0]["k"].run()
    )


def use_class_maps_multi() -> int:
    return (
        ChainMap({"k": A()}, {"j": A()}).maps[0]["k"].execute()
        + ChainMap({"k": B()}, {"j": B()}).maps[0]["k"].run()
    )


def use_class_nc_maps1() -> int:
    return (
        ChainMap({"k": A()}).new_child().maps[1]["k"].execute()
        + ChainMap({"k": B()}).new_child().maps[1]["k"].run()
    )


def use_class_maps_values() -> int:
    return (
        list(ChainMap({"k": A()}).maps[0].values())[0].execute()
        + list(ChainMap({"k": B()}).maps[0].values())[0].run()
    )


# Method-return under foreign same-leaf.
def use_mr_maps_sub(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).maps[0]["k"].execute()
        + ChainMap({"k": bb.get()}).maps[0]["k"].run()
    )


def use_mr_maps_get(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).maps[0].get("k").execute()
        + ChainMap({"k": bb.get()}).maps[0].get("k").run()
    )


def use_mr_maps_assign(ba: BoxA, bb: BoxB) -> int:
    ca = ChainMap({"k": ba.get()})
    cb = ChainMap({"k": bb.get()})
    return ca.maps[0]["k"].execute() + cb.maps[0]["k"].run()


def use_mr_coll_maps(ba: BoxA, bb: BoxB) -> int:
    return (
        collections.ChainMap({"k": ba.get()}).maps[0]["k"].execute()
        + collections.ChainMap({"k": bb.get()}).maps[0]["k"].run()
    )


def use_mr_od_maps(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap(OrderedDict(k=ba.get())).maps[0]["k"].execute()
        + ChainMap(OrderedDict(k=bb.get())).maps[0]["k"].run()
    )


def use_mr_maps_multi(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}, {"j": ba.get()}).maps[0]["k"].execute()
        + ChainMap({"k": bb.get()}, {"j": bb.get()}).maps[0]["k"].run()
    )


def use_mr_nc_maps1(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()}).new_child().maps[1]["k"].execute()
        + ChainMap({"k": bb.get()}).new_child().maps[1]["k"].run()
    )


def use_mr_maps_values(ba: BoxA, bb: BoxB) -> int:
    return (
        list(ChainMap({"k": ba.get()}).maps[0].values())[0].execute()
        + list(ChainMap({"k": bb.get()}).maps[0].values())[0].run()
    )


# Plain ChainMap subscript regression (no maps).
def use_mr_cm_plain(ba: BoxA, bb: BoxB) -> int:
    return (
        ChainMap({"k": ba.get()})["k"].execute()
        + ChainMap({"k": bb.get()})["k"].run()
    )
