class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_dict_kw_sub():
    da = dict(k=[A()])
    db = dict(k=[B()])
    return da["k"][0].execute() + db["k"][0].run()


def use_dict_kw_multi():
    da = dict(k=[A()], m=(A(),))
    db = dict(k=[B()], m=(B(),))
    return da["k"][0].execute() + da["m"][0].execute() + db["k"][0].run() + db["m"][0].run()


def use_dict_kw_match():
    da = dict(k=[A()])
    db = dict(k=[B()])
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_dict_kw_var():
    da = dict(k=[A()])
    db = dict(k=[B()])
    ga = da["k"]
    gb = db["k"]
    return ga[0].execute() + gb[0].run()


def use_dict_kw_for():
    da = dict(k=[A()])
    db = dict(k=[B()])
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_dict_pairs_sub():
    da = dict([("k", [A()])])
    db = dict([("k", [B()])])
    return da["k"][0].execute() + db["k"][0].run()


def use_dict_pairs_tuple():
    da = dict((("k", [A()]),))
    db = dict((("k", [B()]),))
    return da["k"][0].execute() + db["k"][0].run()


def use_dict_pairs_multi():
    da = dict([("k", [A()]), ("m", [A()])])
    db = dict([("k", [B()]), ("m", [B()])])
    return da["k"][0].execute() + da["m"][0].execute() + db["k"][0].run() + db["m"][0].run()


def use_dict_pairs_match():
    da = dict([("k", [A()])])
    db = dict([("k", [B()])])
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_dict_from_literal():
    da = dict({"k": [A()]})
    db = dict({"k": [B()]})
    return da["k"][0].execute() + db["k"][0].run()


def use_set_map_for():
    da = {"k": {A()}}
    db = {"k": {B()}}
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_set_map_next():
    da = {"k": {A()}}
    db = {"k": {B()}}
    return next(iter(da["k"])).execute() + next(iter(db["k"])).run()


def use_set_map_var():
    da = {"k": {A()}}
    db = {"k": {B()}}
    ga = da["k"]
    gb = db["k"]
    return next(iter(ga)).execute() + next(iter(gb)).run()


def use_set_map_values():
    da = {"k": {A()}}
    db = {"k": {B()}}
    n = 0
    for ga in da.values():
        n += next(iter(ga)).execute()
    for gb in db.values():
        n += next(iter(gb)).run()
    return n


def use_dict_kw_set():
    da = dict(k={A()})
    db = dict(k={B()})
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_dict_pairs_set():
    da = dict([("k", {A()})])
    db = dict([("k", {B()})])
    return next(iter(da["k"])).execute() + next(iter(db["k"])).run()



def use_set_map_match():
    da = {"k": {A()}}
    db = {"k": {B()}}
    match da:
        case {"k": s}:
            r = next(iter(s)).execute()
    match db:
        case {"k": t}:
            r += next(iter(t)).run()
    return r

def use_preserves_b():
    db = dict(k=[B()])
    return db["k"][0].run()
