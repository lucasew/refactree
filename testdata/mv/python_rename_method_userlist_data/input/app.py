from collections import UserList
import collections
from collections import UserDict

class A:
    def run(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

def use_data_sub():
    xs = UserList([A()])
    ys = UserList([B()])
    return xs.data[0].run() + ys.data[0].run()

def use_data_var():
    xs = UserList([A()])
    ys = UserList([B()])
    a = xs.data[0]
    b = ys.data[0]
    return a.run() + b.run()

def use_data_for():
    xs = UserList([A()])
    ys = UserList([B()])
    n = 0
    for a in xs.data:
        n += a.run()
    for b in ys.data:
        n += b.run()
    return n

def use_data_local():
    xs = UserList([A()])
    ys = UserList([B()])
    da = xs.data
    db = ys.data
    return da[0].run() + db[0].run()

def use_data_append():
    xs = UserList()
    ys = UserList()
    xs.append(A())
    ys.append(B())
    return xs.data[0].run() + ys.data[0].run()

def use_collections_userlist_data():
    xs = collections.UserList([A()])
    ys = collections.UserList([B()])
    return xs.data[0].run() + ys.data[0].run()

def use_userdict_data():
    da = UserDict(k=A())
    db = UserDict(k=B())
    return da.data["k"].run() + db.data["k"].run()

def use_userdict_data_var():
    da = UserDict(k=A())
    db = UserDict(k=B())
    xa = da.data["k"]
    xb = db.data["k"]
    return xa.run() + xb.run()

def use_userdict_nested_data():
    da = UserDict(k=[A()])
    db = UserDict(k=[B()])
    return da.data["k"][0].run() + db.data["k"][0].run()

def use_userdict_nested_data_local():
    da = UserDict(k=[A()])
    db = UserDict(k=[B()])
    xa = da.data
    xb = db.data
    return xa["k"][0].run() + xb["k"][0].run()

def use_data_pop():
    xs = UserList([A()])
    ys = UserList([B()])
    return xs.data.pop().run() + ys.data.pop().run()


def use_preserves_b():
    ys = UserList([B()])
    return ys.data[0].run()
