from collections import OrderedDict
import collections


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_nested_od_sub():
    da = OrderedDict(outer=OrderedDict(k=A()))
    db = OrderedDict(outer=OrderedDict(k=B()))
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_nested_od_var():
    da = OrderedDict(outer=OrderedDict(k=A()))
    db = OrderedDict(outer=OrderedDict(k=B()))
    ia = da["outer"]
    ib = db["outer"]
    return ia["k"].execute() + ib["k"].run()


def use_nested_od_pairs():
    da = OrderedDict([("outer", OrderedDict(k=A()))])
    db = OrderedDict([("outer", OrderedDict(k=B()))])
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_nested_od_from_literal():
    da = OrderedDict({"outer": OrderedDict(k=A())})
    db = OrderedDict({"outer": OrderedDict(k=B())})
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_nested_dict_od():
    da = dict(outer=OrderedDict(k=A()))
    db = dict(outer=OrderedDict(k=B()))
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_nested_od_dict():
    da = OrderedDict(outer={"k": A()})
    db = OrderedDict(outer={"k": B()})
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_collections_od_nested():
    da = collections.OrderedDict(outer=collections.OrderedDict(k=A()))
    db = collections.OrderedDict(outer=collections.OrderedDict(k=B()))
    return da["outer"]["k"].execute() + db["outer"]["k"].run()


def use_preserves_b():
    db = OrderedDict(outer=OrderedDict(k=B()))
    return db["outer"]["k"].run()
