---
hidden:     false
layout:     post
title:      如何在Java代码里像本地代码一样调用AWS lambda
date:       2018-03-21 15:01:29
summary:    调用lambda就需要trigger? No，Lambda也可以像调用Java API一样简单!
categories: jekyll
---

## 先上代码

假设我们有一个线上`lambda`做ETL的Extract和Load，线上的`lambda`接受以下event:
```
{
    "projectId": 50234,
    "teamId": 69583,
    "forDate": "2018-03-21"
}
```

处理成功`lambda`就返回以下message:
```
{
    "workers": 35,
    "hours": 237.4
}
```

### 如何实现

首先，当然是定义对应的Java接口, 假设`lambda`的名字是`insightsRollup`:
```
public interface InsightService {

    @LambdaFunction(functionName="insightsRollup")
    RollupResponse insightsRollup(InsightsRollupEvent input);

}
```

输入和输出:
```
public class InsightsRollupEvent {

    private Integer projectId;
    private Integer teamId;
    private LocalDate forDate;

    ...

}

public class RollupResponse {
    
    private Integer workers;
    private Double hours;

    ...

}
```

**然后呢？然后就基本完成了！**

接下来的工作可以都交给`LambdaInvokerFactory`来构造`InsightService`的实例:

```
BasicAWSCredentials awsCreds = new BasicAWSCredentials(accessKey, secretKey);
insightService = LambdaInvokerFactory.builder()
        .lambdaClient(
                AWSLambdaClientBuilder.standard()
                        .withRegion(region)
                        .withCredentials(new AWSStaticCredentialsProvider(awsCreds))
                        .build()
        )
        .build(InsightService.class);
```

然后你就可以像本地方法一样使用`insightService.insightService()`调用远端的`insightRollup`.

## 做了什么

`lambda`是部署在aws的服务器上的，我们是如何把请求发送到远端的服务器并接受返回的呢。打开log4j测试一下:

`log4j.rootLogger = DEBUG`

```
@Test
public void insightsRollup() {

    RollupResponse resp = insightService().insightsRollup(new InsightsRollupEvent(
            50234,
            69583,
            LocalDate.now()
    ));

    System.out.println(resp)

}
```

运行，注意到下面：

```
Serialized request object to '{"projectId": 50234,"teamId": 69583,"forDate": "2018-03-21"}'
```

说明数据是通过`json`传输的

还有下面：

```
[com.amazonaws.request]: Sending Request: POST https://lambda.ap-southeast-2.amazonaws.com/2015-03-31/functions/insightsRollup/invocations Headers: (X-Amz-Invocation-Type: RequestResponse, User-Agent: aws-sdk-java/1.11.163 Mac_OS_X/10.12.3 Java_HotSpot(TM)_64-Bit_Server_VM/25.131-b11/1.8.0_131, amz-sdk-invocation-id: 6cc1a52f-dd73-4de4-e249-8c1555db8eea, X-Amz-Log-Type: None, Content-Type: , ) 

```

这其实就是`lambda`的[rest api](https://docs.aws.amazon.com/lambda/latest/dg/API_Invoke.html). 

`LambdaInvokerFactory`做得就是让你用Interface和Annotation的方式，帮你构造一个rest client，达到本地调用`lambda`的目的. 

**注意这种调用方式是同步的**

## 为什么要使用Java同步调用lambda？

最近开始大规模使用`lambda`, 或者说`serverless`以后，这是我经常思考的问题。以前我们讨论SOA，再到后来micro-service，然后现在又出现nano-service. 这些大方向的东西以后再说，单说这种同步调用lambda的方式，我觉得好处就有：

1. 具有`micro/nano service`的好处，你可以使用不同的语言开发`lambda`，分布式的架构迫使你定义清晰的Interface和事务边界，独立部署，独立伸缩，独立发布。

2. 避免了`micro/nano service`架构在基础建设上的overhead. 相比较`api gateway`, `sns`等触发`lambda`的方式, 这种方式明显简单得多, 不需要很多的基础架构设置. 而`lambda`本身具有不需要管理服务器和便宜的好处。

3. `rest`同步的方式避免了分布式异步事务的问题

当然现在看来缺点是性能，But who cares~