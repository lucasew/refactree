from types import MappingProxyType
import types


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


# Scalar-from-MappingProxyType subscript/get assign method-return under foreign same-leaf.
def use_proxy_sub_assign_mr(ba: BoxA, bb: BoxB):
    mrA = MappingProxyType({"k": ba.get()})["k"]
    mrB = MappingProxyType({"k": bb.get()})["k"]
    return mrA.execute() + mrB.run()


def use_proxy_get_assign_mr(ba: BoxA, bb: BoxB):
    mrGA = MappingProxyType({"k": ba.get()}).get("k")
    mrGB = MappingProxyType({"k": bb.get()}).get("k")
    return mrGA.execute() + mrGB.run()


def use_proxy_types_sub_assign_mr(ba: BoxA, bb: BoxB):
    mrTA = types.MappingProxyType({"k": ba.get()})["k"]
    mrTB = types.MappingProxyType({"k": bb.get()})["k"]
    return mrTA.execute() + mrTB.run()


def use_proxy_walrus_mr(ba: BoxA, bb: BoxB):
    if (mrWA := MappingProxyType({"k": ba.get()})["k"]):
        if (mrWB := MappingProxyType({"k": bb.get()})["k"]):
            return mrWA.execute() + mrWB.run()
    return 0


def use_proxy_get_walrus_mr(ba: BoxA, bb: BoxB):
    if (mrWGA := MappingProxyType({"k": ba.get()}).get("k")):
        if (mrWGB := MappingProxyType({"k": bb.get()}).get("k")):
            return mrWGA.execute() + mrWGB.run()
    return 0


# Inline / da-assign already worked.
def use_proxy_inline_mr(ba: BoxA, bb: BoxB):
    return MappingProxyType({"k": ba.get()})["k"].execute() + MappingProxyType({"k": bb.get()})["k"].run()


def use_proxy_da_assign_mr(ba: BoxA, bb: BoxB):
    da = MappingProxyType({"k": ba.get()})
    db = MappingProxyType({"k": bb.get()})
    return da["k"].execute() + db["k"].run()


# Class regression — already worked.
def use_proxy_sub_assign_class():
    classA = MappingProxyType({"k": A()})["k"]
    classB = MappingProxyType({"k": B()})["k"]
    return classA.execute() + classB.run()


def use_proxy_get_assign_class():
    classGA = MappingProxyType({"k": A()}).get("k")
    classGB = MappingProxyType({"k": B()}).get("k")
    return classGA.execute() + classGB.run()


def use_preserves_b(bb: BoxB):
    mrB = MappingProxyType({"k": bb.get()})["k"]
    mrGB = MappingProxyType({"k": bb.get()}).get("k")
    return mrB.run() + mrGB.run()
