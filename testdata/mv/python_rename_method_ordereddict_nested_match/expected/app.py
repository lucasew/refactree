from collections import OrderedDict

class A:
    def execute(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

def use_nested_od_match():
    da = OrderedDict(outer=OrderedDict(k=A()))
    db = OrderedDict(outer=OrderedDict(k=B()))
    match da:
        case {"outer": {"k": xa}}:
            r = xa.execute()
    match db:
        case {"outer": {"k": xb}}:
            r += xb.run()
    return r

def use_nested_od_match_as():
    da = OrderedDict(outer=OrderedDict(k=A()))
    db = OrderedDict(outer=OrderedDict(k=B()))
    match da:
        case {"outer": {"k": xa as x}}:
            r = x.execute()
    match db:
        case {"outer": {"k": xb as y}}:
            r += y.run()
    return r

def use_nested_od_match_inner():
    da = OrderedDict(outer=OrderedDict(k=A()))
    db = OrderedDict(outer=OrderedDict(k=B()))
    match da:
        case {"outer": inner}:
            r = inner["k"].execute()
    match db:
        case {"outer": innerb}:
            r += innerb["k"].run()
    return r

def use_preserves_b():
    db = OrderedDict(outer=OrderedDict(k=B()))
    match db:
        case {"outer": {"k": xb}}:
            return xb.run()
    return 0
