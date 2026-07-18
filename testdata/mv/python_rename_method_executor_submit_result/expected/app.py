from concurrent.futures import ThreadPoolExecutor


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_submit_result():
    with ThreadPoolExecutor() as ex:
        return ex.submit(lambda: A()).result().execute() + ex.submit(lambda: B()).result().run()


def use_preserves_b():
    with ThreadPoolExecutor() as ex:
        return ex.submit(lambda: B()).result().run()
