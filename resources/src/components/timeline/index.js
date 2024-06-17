import React, {useEffect, useRef, useState} from "react";
import './timeline.css';

export default function Timeline({images, setImages}) {
    const [selected, setSelected] = useState(-1);
    const [folders, setFolders] = useState([]);
    const [nameFolders, setNameFolders] = useState([]);
    const [groupedImages, setGroupedImages] = useState([]);
    const [left, setLeft] = useState(0)
    const refDiv = useRef();
    useEffect(() => {
        const listFolders = images.reduce((m, img) => {
            const key = img.folder.substring(0, img.folder.length - 1);
            if (!m.has(key)) {
                m.set(key, [img])
            } else {
                m.get(key).push(img)
            }
            return m
        }, new Map());
        const entries = [...listFolders.entries()].sort((a,b)=>a[1][0] >b[1][0] ? -1:1)

        setNameFolders(entries.map(e=>e[0]).map(f => f.replaceAll('_',' ').substring(f.lastIndexOf("/") + 1).toLowerCase()));
        setGroupedImages(entries.map(e=>e[1]));
    }, [images])

    useEffect(() => {
        if(selected === -1){
            return;
        }
        setImages(groupedImages[selected])
    }, [selected])


    const step = 200;
    const goRight = () => {
        // Stop if end of div and not displayable
        const totalSize = nameFolders.length * 100;
        if(totalSize + left < refDiv.current.offsetWidth){
            return;
        }
        setLeft(left - step);
    }

    const goLeft = () => {
        if (left >= 0) {
            return;
        }
        setLeft(left + step);
    }

    return <div className={"my-timeline"}>

        <div onClick={goLeft}> &#x3c; </div>
        <div ref={refDiv}>
            <div style={{transition:'1s',left: `${left}px`}}>
                <ol>
                    {nameFolders.map((f, i) => <li style={{left: `${i * 100}px`}}>
                        <span className={"text"}>{f}</span>
                        <span className={`bullet ${i === selected ? 'selected' : ''}`}
                              onClick={() => setSelected(i)}></span>
                    </li>)}
                </ol>
            </div>
        </div>
        <div onClick={goRight} style={{left: '96%', position: 'absolute'}}> &#x3e; </div>
        <span className={"line-bullets"}></span>
    </div>
}

